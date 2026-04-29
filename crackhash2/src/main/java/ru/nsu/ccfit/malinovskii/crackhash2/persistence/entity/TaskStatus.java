package ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity;

public enum TaskStatus {
    NEW,              // только создан
    QUEUED,           // отправлен в Rabbit
    QUEUED_PENDING,   // RabbitMQ недоступен
    IN_PROGRESS,      // взят воркером
    DONE,             // выполнен
    FAILED            // превышен retry
}
