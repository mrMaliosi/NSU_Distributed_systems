package ru.nsu.ccfit.malinovskii.crackhash2.services;

import com.rabbitmq.client.Channel;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.stereotype.Component;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.ResultMessage;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskMessage;

import java.io.IOException;

@Component
@RequiredArgsConstructor
@Slf4j
public class WorkerListener {

    private final BruteForceService bruteForceService;
    private final RabbitTemplate rabbitTemplate;

    @RabbitListener(queues = "task.queue")
    public void handleTask(TaskMessage task,
                           Channel channel,
                           org.springframework.amqp.core.Message message) throws IOException {

        try {
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
                    "task.exchange",
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