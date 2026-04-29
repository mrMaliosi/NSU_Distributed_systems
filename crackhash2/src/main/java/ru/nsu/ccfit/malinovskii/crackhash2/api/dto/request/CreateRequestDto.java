package ru.nsu.ccfit.malinovskii.crackhash2.api.dto.request;

import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import lombok.Data;

@Data
public class CreateRequestDto {

    @NotBlank
    private String hash;

    @Min(1)
    private int maxLength;

    @NotBlank
    private String algorithm;

    private String alphabet; // optional
}