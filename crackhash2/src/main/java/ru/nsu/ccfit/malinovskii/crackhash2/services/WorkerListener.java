package ru.nsu.ccfit.malinovskii.crackhash2.services;

import com.rabbitmq.client.Channel;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.core.MessageDeliveryMode;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;
import ru.nsu.ccfit.malinovskii.crackhash2.config.RabbitConfig;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.ResultMessage;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskMessage;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskPart;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskStatus;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.repo.TaskPartRepository;

import java.io.IOException;
import java.time.Instant;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;

@Component
@RequiredArgsConstructor
@Slf4j
@Profile("worker")
public class WorkerListener {
    private final BruteForceService bruteForceService;
    private final RabbitTemplate rabbitTemplate;
    private final TaskPartRepository taskPartRepository;

    /**
     * Должен быть меньше dispatcher TASK_TIMEOUT_MS
     */
    private static final long HEARTBEAT_INTERVAL_MS = 5000;
    private final ScheduledExecutorService scheduler =
            Executors.newScheduledThreadPool(2);

    @RabbitListener(queues = "task.queue")
    public void handleTask(
            TaskMessage task,
            Channel channel,
            org.springframework.amqp.core.Message message
    ) throws IOException {
        long deliveryTag = message.getMessageProperties().getDeliveryTag();
        AtomicBoolean running = new AtomicBoolean(true);
        ScheduledFuture<?> heartbeatFuture = null;
        try {
            TaskPart part = taskPartRepository.findById(task.getTaskId()).orElse(null);
            /**
             * Задача уже удалена/не существует
             */
            if (part == null) {
                log.warn("Task part {} not found", task.getTaskId());
                channel.basicAck(deliveryTag, false);
                return;
            }

            /**
             * Duplicate delivery protection
             */
            if (part.getStatus() == TaskStatus.DONE || part.getStatus() == TaskStatus.FAILED) {
                log.info("Task {} already completed with status {}", task.getTaskId(), part.getStatus());
                channel.basicAck(deliveryTag, false);
                return;
            }

            /**
             * Dispatcher должен отправлять только QUEUED
             */
            if (part.getStatus() != TaskStatus.QUEUED && part.getStatus() != TaskStatus.IN_PROGRESS) {
                log.warn(
                        "Task {} in unexpected status {}, ACK",
                        task.getTaskId(),
                        part.getStatus()
                );
                channel.basicAck(deliveryTag, false);
                return;
            }

            /**
             * Claim task
             */
            part.setStatus(TaskStatus.IN_PROGRESS);
            part.setLastUpdated(now());
            taskPartRepository.save(part);

            /**
             * Heartbeat
             */
            heartbeatFuture =
                    scheduler.scheduleAtFixedRate(
                            () -> sendHeartbeat(task.getTaskId(), running),
                            HEARTBEAT_INTERVAL_MS,
                            HEARTBEAT_INTERVAL_MS,
                            TimeUnit.MILLISECONDS
                    );
            log.info("Processing task {}", task.getTaskId());

            /**
             * Brute force execution
             */
            String result = bruteForceService.crack(task);
            running.set(false);
            if (heartbeatFuture != null) {
                heartbeatFuture.cancel(true);
            }

            /**
             * Проверяем актуальное состояние.
             *
             * Dispatcher мог уже ретрайнуть задачу.
             */
            TaskPart current = taskPartRepository.findById(task.getTaskId()).orElse(null);
            if (current == null) {
                log.warn("Task {} disappeared during processing", task.getTaskId());
                channel.basicAck(deliveryTag, false);
                return;
            }

            /**
             * stale worker protection
             */
            if (current.getStatus() == TaskStatus.FAILED || current.getStatus() == TaskStatus.NEW) {
                log.warn("Task {} became stale during execution (status={})", task.getTaskId(), current.getStatus());
                channel.basicAck(deliveryTag, false);
                return;
            }

            /**
             * Publish result
             */
            ResultMessage response = new ResultMessage();
            response.setTaskId(task.getTaskId());
            response.setRequestId(task.getRequestId());
            if (result != null && !result.isBlank()) {
                response.setFound(true);
                response.setResult(result);
            } else {
                response.setFound(false);
            }

            rabbitTemplate.convertAndSend(
                    RabbitConfig.EXCHANGE,
                    "result",
                    response,
                    msg -> {
                        msg.getMessageProperties()
                                .setDeliveryMode(
                                        MessageDeliveryMode.PERSISTENT
                                );
                        return msg;
                    }
            );
            log.info("Task {} completed and result published", task.getTaskId());

            /**
             * ACK only after successful publish
             */
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            running.set(false);
            if (heartbeatFuture != null) {
                heartbeatFuture.cancel(true);
            }
            log.error("Worker failed task {}", task.getTaskId(), e);

            /**
             * Retry message delivery
             */
            channel.basicNack(deliveryTag, false, true);
        }
    }

    /**
     * Lease heartbeat
     */
    private void sendHeartbeat(String taskId, AtomicBoolean running) {
        if (!running.get()) {
            return;
        }
        try {
            TaskPart part = taskPartRepository.findById(taskId).orElse(null);
            if (part == null) {
                return;
            }

            /**
             * heartbeat only for active tasks
             */
            if (part.getStatus() != TaskStatus.IN_PROGRESS) {
                return;
            }
            part.setLastUpdated(now());
            taskPartRepository.save(part);
        } catch (Exception e) {
            log.warn("Heartbeat failed for task {}", taskId, e);
        }
    }

    private long now() {
        return Instant.now().toEpochMilli();
    }
}