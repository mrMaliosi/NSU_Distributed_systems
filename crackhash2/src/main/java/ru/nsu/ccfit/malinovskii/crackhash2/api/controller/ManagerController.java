package ru.nsu.ccfit.malinovskii.crackhash2.api.controller;

import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;
import ru.nsu.ccfit.malinovskii.crackhash2.api.dto.request.CreateRequestDto;
import ru.nsu.ccfit.malinovskii.crackhash2.api.dto.response.CreateResponseDto;
import ru.nsu.ccfit.malinovskii.crackhash2.api.dto.response.MetricsResponseDto;
import ru.nsu.ccfit.malinovskii.crackhash2.api.dto.response.StatusResponseDto;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.HashRequest;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.RequestStatus;
import ru.nsu.ccfit.malinovskii.crackhash2.services.ManagerService;
import ru.nsu.ccfit.malinovskii.crackhash2.utils.SplitterUtils;


import java.util.List;

@RestController
@RequestMapping("/api")
@RequiredArgsConstructor
@Slf4j
public class ManagerController {

    private final ManagerService managerService;

    private static final String DEFAULT_ALPHABET = "abcdefghijklmnopqrstuvwxyz0123456789";

    @PostMapping("/hash/crack")
    public CreateResponseDto crackHash(@Valid @RequestBody CreateRequestDto dto) {

        String alphabet = (dto.getAlphabet() == null || dto.getAlphabet().isEmpty())
                ? DEFAULT_ALPHABET
                : dto.getAlphabet();

        HashRequest request = managerService.createRequest(
                dto.getHash(),
                dto.getMaxLength(),
                dto.getAlgorithm(),
                alphabet
        );

        long estimated = SplitterUtils.estimate(alphabet.length(), dto.getMaxLength());

        return new CreateResponseDto(request.getId(), estimated);
    }

    @GetMapping("/hash/status")
    public StatusResponseDto getStatus(@RequestParam String requestId) {

        HashRequest request = managerService.getStatus(requestId);

        return mapToStatusDto(request);
    }

    @DeleteMapping("/hash/crack")
    public StatusResponseDto cancel(@RequestParam String requestId) {

        HashRequest request = managerService.cancelRequest(requestId);

        return mapToStatusDto(request);
    }

    @GetMapping("/metrics")
    public MetricsResponseDto metrics() {

        var m = managerService.getMetrics();

        return new MetricsResponseDto(
                m.totalTasks(),
                m.activeTasks(),
                m.completedTasks()
        );
    }

    private StatusResponseDto mapToStatusDto(HashRequest request) {

        if (request.getStatus() == RequestStatus.READY) {
            return StatusResponseDto.builder()
                    .status("READY")
                    .data(request.getResult())
                    .build();
        }

        if (request.getStatus() == RequestStatus.ERROR) {
            return StatusResponseDto.builder()
                    .status("ERROR")
                    .error("Computation failed")
                    .build();
        }

        if (request.getStatus() == RequestStatus.CANCELLED) {
            return StatusResponseDto.builder()
                    .status("CANCELLED")
                    .build();
        }

        return StatusResponseDto.builder()
                .status("IN_PROGRESS")
                .data(null)
                .build();
    }
}