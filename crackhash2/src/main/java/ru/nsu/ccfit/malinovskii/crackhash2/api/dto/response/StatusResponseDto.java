package ru.nsu.ccfit.malinovskii.crackhash2.api.dto.response;

import lombok.Builder;
import lombok.Data;

import java.util.List;

@Data
@Builder
public class StatusResponseDto {
    private String status;
    private List<String> data;
    private String error;
}