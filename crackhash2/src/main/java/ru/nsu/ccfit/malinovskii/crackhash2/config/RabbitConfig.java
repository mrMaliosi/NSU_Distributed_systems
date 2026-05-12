package ru.nsu.ccfit.malinovskii.crackhash2.config;

import org.springframework.amqp.core.*;
import org.springframework.amqp.support.converter.JacksonJsonMessageConverter;
import org.springframework.amqp.support.converter.MessageConverter;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;

@Configuration
@Profile({"dispatcher", "worker"})
public class RabbitConfig {

    public static final String TASK_QUEUE = "task.queue";
    public static final String RESULT_QUEUE = "result.queue";
    public static final String EXCHANGE = "task.exchange";

    @Bean
    public Queue taskQueue() {
        return QueueBuilder.durable(TASK_QUEUE).build();
    }

    @Bean
    public Queue resultQueue() {
        return QueueBuilder.durable(RESULT_QUEUE).build();
    }

    @Bean
    public DirectExchange exchange() {
        return new DirectExchange(EXCHANGE, true, false);
    }

    @Bean
    public Binding taskBinding() {
        return BindingBuilder.bind(taskQueue())
                .to(exchange())
                .with("task");
    }

    @Bean
    public Binding resultBinding() {
        return BindingBuilder.bind(resultQueue())
                .to(exchange())
                .with("result");
    }

    @Bean
    public MessageConverter rabbitMessageConverter() {
        return new JacksonJsonMessageConverter();
    }
}