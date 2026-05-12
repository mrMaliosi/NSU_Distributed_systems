package ru.nsu.ccfit.malinovskii.crackhash2.services;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;
import ru.nsu.ccfit.malinovskii.crackhash2.config.RabbitConfig;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.ResultMessage;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskPart;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskStatus;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.repo.TaskPartRepository;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.rabbit.annotation.RabbitListener;

import java.io.IOException;
import java.time.Instant;


@Component
@RequiredArgsConstructor
@Slf4j
@Profile("dispatcher")
public class ResultListener {

    private final TaskPartRepository taskPartRepository;

    @RabbitListener(queues = RabbitConfig.RESULT_QUEUE)
    public void handleResult(ResultMessage msg, Channel channel,
                             org.springframework.amqp.core.Message message) throws IOException {

        try {
            TaskPart part = taskPartRepository.findById(msg.getTaskId())
                    .orElseThrow();

            if (part.getStatus() == TaskStatus.DONE) {
                channel.basicAck(message.getMessageProperties().getDeliveryTag(), false);
                return;
            }

            if (msg.isFound()) {
                part.setResult(msg.getResult());
            }

            part.setStatus(TaskStatus.DONE);
            part.setLastUpdated(Instant.now().toEpochMilli());

            taskPartRepository.save(part);

            channel.basicAck(message.getMessageProperties().getDeliveryTag(), false);

        } catch (Exception e) {
            log.error("Error processing result", e);

            channel.basicNack(message.getMessageProperties().getDeliveryTag(), false, true);
        }
    }
}
