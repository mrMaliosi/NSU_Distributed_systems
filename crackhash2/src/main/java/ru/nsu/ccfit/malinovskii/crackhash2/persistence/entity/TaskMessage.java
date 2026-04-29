package ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity;

import lombok.Data;

@Data
public class TaskMessage {
    private String taskId;
    private String requestId;

    private long rangeStart;
    private long rangeEnd;

    private String hash;
    private String algorithm;
    private String alphabet;
    private int maxLength;
}