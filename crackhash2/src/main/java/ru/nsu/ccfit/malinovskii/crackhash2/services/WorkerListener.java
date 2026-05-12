package ru.nsu.ccfit.malinovskii.crackhash2.services;

import com.rabbitmq.client.Channel;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;
import ru.nsu.ccfit.malinovskii.crackhash2.config.RabbitConfig;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskPart;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.ResultMessage;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskMessage;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskStatus;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.repo.TaskPartRepository;

import java.io.IOException;
import java.time.Instant;

@Component
@RequiredArgsConstructor
@Slf4j
@Profile("worker")
public class WorkerListener {

    private final BruteForceService bruteForceService;
    private final RabbitTemplate rabbitTemplate;
    private final TaskPartRepository taskPartRepository;

    @RabbitListener(queues = "task.queue")
    public void handleTask(TaskMessage task,
                           Channel channel,
                           org.springframework.amqp.core.Message message) throws IOException {

        try {
            TaskPart part = taskPartRepository.findById(task.getTaskId()).orElse(null);
            if (part == null) {
                log.warn("Task part {} not found, ACK message", task.getTaskId());
                channel.basicAck(message.getMessageProperties().getDeliveryTag(), false);
                return;
            }

            if (part.getStatus() == TaskStatus.DONE) {
                // Idempotency: result already saved, duplicate delivery can be acknowledged safely.
                channel.basicAck(message.getMessageProperties().getDeliveryTag(), false);
                return;
            }

            part.setStatus(TaskStatus.IN_PROGRESS);
            part.setLastUpdated(Instant.now().toEpochMilli());
            taskPartRepository.save(part);

            log.info("Processing task {}", task.getTaskId());

            String result = bruteForceService.crack(task);

            ResultMessage response = new ResultMessage();
            response.setTaskId(task.getTaskId());
            response.setRequestId(task.getRequestId());

            if (result != null) {
                response.setFound(true);
                response.setResult(result);
            } else {
                response.setFound(false);
            }

            rabbitTemplate.convertAndSend(
                    RabbitConfig.EXCHANGE,
                    "result",
                    response,
                    message1 -> {
                        message1.getMessageProperties().setDeliveryMode(org.springframework.amqp.core.MessageDeliveryMode.PERSISTENT);
                        return message1;
                    }
            );

            // ✅ ACK только после успешной отправки результата
            channel.basicAck(message.getMessageProperties().getDeliveryTag(), false);

        } catch (Exception e) {
            log.error("Worker failed task {}", task.getTaskId(), e);

            // ❗ requeue → другой воркер попробует
            channel.basicNack(message.getMessageProperties().getDeliveryTag(), false, true);
        }
    }
}