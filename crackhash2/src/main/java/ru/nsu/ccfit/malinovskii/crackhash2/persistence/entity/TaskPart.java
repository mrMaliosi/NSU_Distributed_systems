package ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity;

import lombok.Data;
import org.springframework.data.mongodb.core.index.Indexed;
import org.springframework.data.mongodb.core.mapping.Document;

@Data
@Document("task_parts")
public class TaskPart {

    private String id;

    @Indexed
    private String requestId;

    private long rangeStart;
    private long rangeEnd;

    private TaskStatus status;

    private int attempt; // для retry

    private Long lastUpdated;

    private String result; // если найдено
}