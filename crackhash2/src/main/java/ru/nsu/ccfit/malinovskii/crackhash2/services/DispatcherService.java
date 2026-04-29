package ru.nsu.ccfit.malinovskii.crackhash2.services;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
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
public class DispatcherService {
    private final RabbitTemplate rabbitTemplate;
    private final BruteForceService bruteForceService;

    private final RequestRepository requestRepository;
    private final TaskPartRepository taskPartRepository;

    private static final int PARTS = 10; // пока фикс, потом сделаем динамику
    private static final long PENDING_RETRY_DELAY_MS = 15_000;
    private static final int MAX_ATTEMPTS = 5;
    @Value("${app.dispatch.local-fallback:true}")
    private boolean localFallbackEnabled;

    /**
     * Главный цикл диспетчера
     */
    @Scheduled(fixedDelay = 2000)
    public void dispatchLoop() {
        processNewRequests();
        dispatchTasks();
        retryStuckTasks();
        checkCompletion();
    }

    /**
     * 1. Создание TaskParts
     */
    private void processNewRequests() {
        List<HashRequest> requests = requestRepository.findByStatus(RequestStatus.IN_PROGRESS);

        for (HashRequest request : requests) {

            long existingParts = taskPartRepository.countByRequestId(request.getId());

            if (existingParts > 0) continue;

            log.info("Splitting request {}", request.getId());

            createParts(request);
        }
    }

    private void createParts(HashRequest request) {

        long total = estimateTotal(request);
        long chunk = total / PARTS;

        for (int i = 0; i < PARTS; i++) {
            TaskPart part = new TaskPart();

            part.setId(UUID.randomUUID().toString());
            part.setRequestId(request.getId());

            part.setRangeStart(i * chunk);
            part.setRangeEnd((i + 1) * chunk);

            part.setStatus(TaskStatus.NEW);
            part.setAttempt(0);
            part.setLastUpdated(Instant.now().toEpochMilli());

            taskPartRepository.save(part);
        }
    }

    /**
     * 2. Retry зависших задач
     */
    private void retryStuckTasks() {

        List<TaskPart> stuck = taskPartRepository.findByStatus(TaskStatus.IN_PROGRESS);

        long now = Instant.now().toEpochMilli();

        for (TaskPart part : stuck) {
            if (now - part.getLastUpdated() > 10000) { // 10 секунд

                log.warn("Retrying task {}", part.getId());
                int nextAttempt = part.getAttempt() + 1;
                part.setAttempt(nextAttempt);
                if (nextAttempt >= MAX_ATTEMPTS) {
                    part.setStatus(TaskStatus.FAILED);
                    log.error("Task {} exceeded max attempts", part.getId());
                } else {
                    part.setStatus(TaskStatus.NEW);
                }
                part.setLastUpdated(now);

                taskPartRepository.save(part);
            }
        }
    }

    /**
     * 3. Завершение request
     */
    private void checkCompletion() {

        List<HashRequest> requests = requestRepository.findByStatus(RequestStatus.IN_PROGRESS);

        for (HashRequest request : requests) {

            long total = taskPartRepository.findByRequestId(request.getId()).size();
            long done = taskPartRepository.countByRequestIdAndStatus(
                    request.getId(), TaskStatus.DONE
            );
            long failed = taskPartRepository.countByRequestIdAndStatus(
                    request.getId(), TaskStatus.FAILED
            );

            if (total > 0 && total == done) {

                log.info("HashRequest completed {}", request.getId());
                List<String> results = taskPartRepository.findByRequestId(request.getId()).stream()
                        .map(TaskPart::getResult)
                        .filter(r -> r != null && !r.isBlank())
                        .distinct()
                        .toList();

                request.setStatus(RequestStatus.READY);
                request.setResult(results);
                request.setUpdatedAt(Instant.now().toEpochMilli());

                requestRepository.save(request);
                continue;
            }

            if (failed > 0) {
                request.setStatus(RequestStatus.ERROR);
                request.setUpdatedAt(Instant.now().toEpochMilli());
                requestRepository.save(request);
            }
        }
    }

    private long estimateTotal(HashRequest request) {
        int alphabet = request.getAlphabet().length();
        int max = request.getMaxLength();

        long total = 0;
        for (int i = 1; i <= max; i++) {
            total += Math.pow(alphabet, i);
        }

        return total;
    }

    private void sendToQueue(TaskPart part, HashRequest request) {
        TaskMessage msg = buildMessage(part, request);
        try {
            rabbitTemplate.convertAndSend(
                    RabbitConfig.EXCHANGE,
                    "task",
                    msg,
                    message -> {
                        message.getMessageProperties().setDeliveryMode(org.springframework.amqp.core.MessageDeliveryMode.PERSISTENT);
                        return message;
                    }
            );

            if (part.getStatus() == TaskStatus.QUEUED_PENDING) {
                log.info("RabbitMQ recovered, task {} sent", part.getId());
            }
            part.setStatus(TaskStatus.QUEUED);

        } catch (Exception e) {
            if (part.getStatus() != TaskStatus.QUEUED_PENDING) {
                log.error("Failed to publish task {} to RabbitMQ; task queued pending", part.getId(), e);
            }

            if (localFallbackEnabled) {
                processTaskLocally(part, msg);
                return;
            } else {
                part.setStatus(TaskStatus.QUEUED_PENDING);
            }
        }

        part.setLastUpdated(System.currentTimeMillis());
        taskPartRepository.save(part);
    }

    private TaskMessage buildMessage(TaskPart part, HashRequest request) {
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

    private void processTaskLocally(TaskPart part, TaskMessage msg) {
        try {
            String result = bruteForceService.crack(msg);
            part.setResult(result);
            part.setStatus(TaskStatus.DONE);
            part.setLastUpdated(System.currentTimeMillis());
            taskPartRepository.save(part);
            log.info("Task {} processed locally", part.getId());
        } catch (Exception localEx) {
            part.setStatus(TaskStatus.QUEUED_PENDING);
            part.setLastUpdated(System.currentTimeMillis());
            taskPartRepository.save(part);
            log.error("Local fallback failed for task {}", part.getId(), localEx);
        }
    }

    private void dispatchTasks() {
        List<TaskPart> newTasks = taskPartRepository.findByStatusIn(
                List.of(TaskStatus.NEW, TaskStatus.QUEUED_PENDING)
        );
        long now = System.currentTimeMillis();

        for (TaskPart part : newTasks) {
            if (part.getStatus() == TaskStatus.QUEUED_PENDING
                    && part.getLastUpdated() != null
                    && now - part.getLastUpdated() < PENDING_RETRY_DELAY_MS) {
                continue;
            }
            HashRequest request = requestRepository.findById(part.getRequestId()).orElseThrow();
            if (request.getStatus() != RequestStatus.IN_PROGRESS) {
                continue;
            }
            sendToQueue(part, request);
        }
    }
}
