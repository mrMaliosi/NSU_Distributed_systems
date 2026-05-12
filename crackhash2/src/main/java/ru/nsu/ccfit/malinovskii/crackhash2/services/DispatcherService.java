package ru.nsu.ccfit.malinovskii.crackhash2.services;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.core.MessageDeliveryMode;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Profile;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import ru.nsu.ccfit.malinovskii.crackhash2.config.RabbitConfig;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.*;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.repo.RequestRepository;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.repo.TaskPartRepository;

import java.time.Instant;
import java.util.List;
import java.util.UUID;

@Service
@RequiredArgsConstructor
@Slf4j
@Profile("dispatcher")
public class DispatcherService {

    private final RabbitTemplate rabbitTemplate;
    private final BruteForceService bruteForceService;

    private final RequestRepository requestRepository;
    private final TaskPartRepository taskPartRepository;

    private static final int PARTS = 10;

    /**
     * Сколько ждём перед повторной отправкой QUEUED_PENDING
     */
    private static final long PENDING_RETRY_DELAY_MS = 15_000;

    /**
     * Timeout lease задачи
     */
    private static final long TASK_TIMEOUT_MS = 30_000;

    private static final int MAX_ATTEMPTS = 5;

    @Value("${app.dispatch.local-fallback:true}")
    private boolean localFallbackEnabled;

    /**
     * Главный цикл диспетчера
     */
    @Scheduled(fixedDelay = 2000)
    public void dispatchLoop() {

        try {
            processNewRequests();
            dispatchTasks();
            retryExpiredTasks();
            checkCompletion();
        } catch (Exception e) {
            log.error("Dispatcher loop failed", e);
        }
    }

    /**
     * Создание частей request
     */
    private void processNewRequests() {
        List<HashRequest> requests =
                requestRepository.findByStatus(RequestStatus.IN_PROGRESS);
        for (HashRequest request : requests) {
            long existing =
                    taskPartRepository.countByRequestId(request.getId());
            if (existing > 0) {
                continue;
            }
            log.info("Creating task parts for request {}", request.getId());
            createParts(request);
        }
    }

    /**
     * Корректное деление диапазона
     */
    @Transactional
    protected void createParts(HashRequest request) {
        long total = estimateTotal(request);
        long chunk = Math.max(1, total / PARTS);
        long start = 0;
        for (int i = 0; i < PARTS && start < total; i++) {
            long end =
                    (i == PARTS - 1)
                            ? total
                            : Math.min(total, start + chunk);

            TaskPart part = new TaskPart();
            part.setId(UUID.randomUUID().toString());
            part.setRequestId(request.getId());
            part.setRangeStart(start);
            part.setRangeEnd(end);
            part.setStatus(TaskStatus.NEW);
            part.setAttempt(0);
            part.setLastUpdated(now());
            taskPartRepository.save(part);
            start = end;
        }
    }

    /**
     * Отправка новых задач
     */
    private void dispatchTasks() {
        List<TaskPart> tasks =
                taskPartRepository.findByStatusIn(
                        List.of(
                                TaskStatus.NEW,
                                TaskStatus.QUEUED_PENDING
                        )
                );
        long now = now();
        for (TaskPart part : tasks) {
            if (part.getStatus() == TaskStatus.QUEUED_PENDING
                    && part.getLastUpdated() != null
                    && now - part.getLastUpdated() < PENDING_RETRY_DELAY_MS) {
                continue;
            }
            HashRequest request =
                    requestRepository.findById(part.getRequestId())
                            .orElse(null);
            if (request == null) {
                continue;
            }
            if (request.getStatus() != RequestStatus.IN_PROGRESS) {
                continue;
            }
            sendToQueue(part, request);
        }
    }

    /**
     * Retry зависших задач
     *
     * QUEUED и IN_PROGRESS считаются lease-based состояниями.
     */
    private void retryExpiredTasks() {
        List<TaskPart> active =
                taskPartRepository.findByStatusIn(
                        List.of(
                                TaskStatus.QUEUED,
                                TaskStatus.IN_PROGRESS
                        )
                );
        long now = now();
        for (TaskPart part : active) {
            if (part.getStatus() == TaskStatus.DONE
                    || part.getStatus() == TaskStatus.FAILED) {
                continue;
            }
            if (part.getLastUpdated() == null) {
                continue;
            }
            long age = now - part.getLastUpdated();
            if (age < TASK_TIMEOUT_MS) {
                continue;
            }
            int nextAttempt = part.getAttempt() + 1;
            log.warn(
                    "Task {} expired (status={}), retry attempt {}",
                    part.getId(),
                    part.getStatus(),
                    nextAttempt
            );
            part.setAttempt(nextAttempt);
            if (nextAttempt >= MAX_ATTEMPTS) {
                part.setStatus(TaskStatus.FAILED);
                log.error(
                        "Task {} exceeded max attempts",
                        part.getId()
                );
            } else {
                part.setStatus(TaskStatus.NEW);
            }
            part.setLastUpdated(now);
            taskPartRepository.save(part);
        }
    }

    /**
     * Проверка завершения request
     */
    private void checkCompletion() {
        List<HashRequest> requests =
                requestRepository.findByStatus(RequestStatus.IN_PROGRESS);
        for (HashRequest request : requests) {
            List<TaskPart> parts =
                    taskPartRepository.findByRequestId(request.getId());

            if (parts.isEmpty()) {
                continue;
            }
            long total = parts.size();
            long done =
                    parts.stream()
                            .filter(p -> p.getStatus() == TaskStatus.DONE)
                            .count();
            long failed =
                    parts.stream()
                            .filter(p -> p.getStatus() == TaskStatus.FAILED)
                            .count();

            if (failed > 0) {
                log.error(
                        "Request {} failed",
                        request.getId()
                );
                request.setStatus(RequestStatus.ERROR);
                request.setUpdatedAt(now());
                requestRepository.save(request);
                continue;
            }

            if (done == total) {
                List<String> results =
                        parts.stream()
                                .map(TaskPart::getResult)
                                .filter(r -> r != null && !r.isBlank())
                                .distinct()
                                .toList();
                log.info(
                        "Request {} completed",
                        request.getId()
                );
                request.setStatus(RequestStatus.READY);
                request.setResult(results);
                request.setUpdatedAt(now());
                requestRepository.save(request);
            }
        }
    }

    /**
     * Отправка в RabbitMQ
     */
    private void sendToQueue(
            TaskPart part,
            HashRequest request
    ) {
        if (part.getStatus() == TaskStatus.DONE
                || part.getStatus() == TaskStatus.FAILED) {
            return;
        }
        TaskMessage msg = buildMessage(part, request);
        try {
            /**
             * Сначала помечаем как QUEUED,
             * потом отправляем.
             */
            part.setStatus(TaskStatus.QUEUED);
            part.setLastUpdated(now());
            taskPartRepository.save(part);
            rabbitTemplate.convertAndSend(
                    RabbitConfig.EXCHANGE,
                    "task",
                    msg,
                    message -> {
                        message.getMessageProperties()
                                .setDeliveryMode(
                                        MessageDeliveryMode.PERSISTENT
                                );
                        return message;
                    }
            );
            log.info(
                    "Task {} sent to RabbitMQ",
                    part.getId()
            );
        } catch (Exception e) {
            log.error(
                    "Failed to publish task {}",
                    part.getId(),
                    e
            );

            if (localFallbackEnabled) {
                processTaskLocally(part, msg);
                return;
            }

            part.setStatus(TaskStatus.QUEUED_PENDING);
            part.setLastUpdated(now());
            taskPartRepository.save(part);
        }
    }

    /**
     * Local fallback execution
     */
    private void processTaskLocally(
            TaskPart part,
            TaskMessage msg
    ) {
        try {
            part.setStatus(TaskStatus.IN_PROGRESS);
            part.setLastUpdated(now());
            taskPartRepository.save(part);
            String result = bruteForceService.crack(msg);
            /**
             * IMPORTANT:
             * не перезаписываем FAILED
             */
            TaskPart current =
                    taskPartRepository.findById(part.getId())
                            .orElseThrow();
            if (current.getStatus() == TaskStatus.FAILED) {
                log.warn(
                        "Task {} already failed, ignoring local result",
                        part.getId()
                );
                return;
            }
            current.setResult(result);
            current.setStatus(TaskStatus.DONE);
            current.setLastUpdated(now());
            taskPartRepository.save(current);
            log.info(
                    "Task {} processed locally",
                    part.getId()
            );
        } catch (Exception e) {
            log.error(
                    "Local execution failed for task {}",
                    part.getId(),
                    e
            );
            part.setStatus(TaskStatus.QUEUED_PENDING);
            part.setLastUpdated(now());
            taskPartRepository.save(part);
        }
    }

    /**
     * Build MQ message
     */
    private TaskMessage buildMessage(
            TaskPart part,
            HashRequest request
    ) {
        TaskMessage msg = new TaskMessage();
        msg.setTaskId(part.getId());
        msg.setRequestId(request.getId());
        msg.setRangeStart(part.getRangeStart());
        msg.setRangeEnd(part.getRangeEnd());
        msg.setHash(request.getHash());
        msg.setAlgorithm(request.getAlgorithm());
        msg.setAlphabet(request.getAlphabet());
        msg.setMaxLength(request.getMaxLength());
        return msg;
    }

    /**
     * Оценка total search space
     */
    private long estimateTotal(HashRequest request) {
        int alphabet = request.getAlphabet().length();
        int max = request.getMaxLength();
        long total = 0;
        for (int i = 1; i <= max; i++) {
            total += (long) Math.pow(alphabet, i);
        }

        return total;
    }

    private long now() {
        return Instant.now().toEpochMilli();
    }
}