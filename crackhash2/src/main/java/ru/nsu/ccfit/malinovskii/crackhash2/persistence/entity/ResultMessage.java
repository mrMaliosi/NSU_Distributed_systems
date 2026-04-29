package ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity;

import lombok.Data;

@Data
public class ResultMessage {
    private String taskId;
    private String requestId;

    private boolean found;
    private String result;
}