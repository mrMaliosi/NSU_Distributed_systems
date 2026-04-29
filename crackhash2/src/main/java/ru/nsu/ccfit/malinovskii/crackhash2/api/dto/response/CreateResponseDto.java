package ru.nsu.ccfit.malinovskii.crackhash2.api.dto.response;

import lombok.AllArgsConstructor;
import lombok.Data;

@Data
@AllArgsConstructor
public class CreateResponseDto {
    private String requestId;
    private long estimatedCombinations;
}