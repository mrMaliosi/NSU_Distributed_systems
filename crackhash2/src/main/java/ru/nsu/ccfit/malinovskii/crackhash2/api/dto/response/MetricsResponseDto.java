package ru.nsu.ccfit.malinovskii.crackhash2.api.dto.response;

import lombok.AllArgsConstructor;
import lombok.Data;

@Data
@AllArgsConstructor
public class MetricsResponseDto {
    private long totalTasks;
    private long activeTasks;
    private long completedTasks;
}