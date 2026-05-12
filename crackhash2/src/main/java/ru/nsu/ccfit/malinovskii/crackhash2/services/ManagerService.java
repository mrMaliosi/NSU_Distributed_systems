package ru.nsu.ccfit.malinovskii.crackhash2.services;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.stereotype.Service;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.HashRequest;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.RequestStatus;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.repo.RequestRepository;
import ru.nsu.ccfit.malinovskii.crackhash2.utils.SplitterUtils;

import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

@Service
@Slf4j
@RequiredArgsConstructor
@Profile("manager")
public class ManagerService {

    private final RequestRepository requestRepository;

    /**
     * Создание запроса (с идемпотентностью)
     */
    public HashRequest createRequest(String hash,
                                     int maxLength,
                                     String algorithm,
                                     String alphabet) {

        log.info("Create request: hash={}, maxLength={}, algorithm={}", hash, maxLength, algorithm);

        // 1. Проверка идемпотентности
        Optional<HashRequest> existing = requestRepository
                .findByHashAndMaxLengthAndAlgorithmAndAlphabet(hash, maxLength, algorithm, alphabet);

        if (existing.isPresent()) {
            HashRequest req = existing.get();
            log.info("Request already exists: id={}, status={}", req.getId(), req.getStatus());
            return req;
        }

        // 2. Создание нового запроса
        HashRequest request = new HashRequest();
        request.setId(UUID.randomUUID().toString());
        request.setHash(hash);
        request.setMaxLength(maxLength);
        request.setAlgorithm(algorithm);
        request.setAlphabet(alphabet);
        request.setStatus(RequestStatus.IN_PROGRESS);
        request.setResult(null);
        long now = Instant.now().toEpochMilli();
        request.setCreatedAt(now);
        request.setUpdatedAt(now);

        // 3. Сохранение (с защитой от race condition)
        try {
            requestRepository.save(request);
            log.info("Request saved: id={}", request.getId());
        } catch (DuplicateKeyException e) {
            // Кто-то успел вставить раньше → повторно читаем
            log.warn("Duplicate request detected, re-fetching");

            return requestRepository
                    .findByHashAndMaxLengthAndAlgorithmAndAlphabet(hash, maxLength, algorithm, alphabet)
                    .orElseThrow();
        }

        return request;
    }

    /**
     * Получение статуса
     */
    public HashRequest getStatus(String requestId) {
        log.info("Get status: requestId={}", requestId);

        return requestRepository.findById(requestId)
                .orElseThrow(() -> new RuntimeException("Request not found"));
    }

    /**
     * Отмена запроса
     */
    public HashRequest cancelRequest(String requestId) {
        log.info("Cancel request: requestId={}", requestId);

        HashRequest request = requestRepository.findById(requestId)
                .orElseThrow(() -> new RuntimeException("Request not found"));

        if (request.getStatus() == RequestStatus.READY ||
                request.getStatus() == RequestStatus.ERROR) {

            log.warn("Request already finished: id={}", requestId);
            return request;
        }

        request.setStatus(RequestStatus.CANCELLED);
        request.setUpdatedAt(Instant.now().toEpochMilli());

        requestRepository.save(request);

        return request;
    }

    /**
     * Простейшие метрики
     */
    public Metrics getMetrics() {
        long total = requestRepository.count();
        long completed = requestRepository.countByStatus(RequestStatus.READY);
        long active = requestRepository.countByStatus(RequestStatus.IN_PROGRESS);
        long avgSpeed = calculateAvgSpeedWordsPerSec();

        return new Metrics(total, active, completed, avgSpeed);
    }

    private long calculateAvgSpeedWordsPerSec() {
        var readyRequests = requestRepository.findByStatus(RequestStatus.READY);
        long processed = 0;
        long durationMillis = 0;

        for (HashRequest request : readyRequests) {
            if (request.getCreatedAt() == null || request.getUpdatedAt() == null) {
                continue;
            }

            long requestDuration = request.getUpdatedAt() - request.getCreatedAt();
            if (requestDuration <= 0) {
                continue;
            }

            processed += SplitterUtils.estimate(request.getAlphabet().length(), request.getMaxLength());
            durationMillis += requestDuration;
        }

        if (processed == 0 || durationMillis == 0) {
            return 0;
        }

        return (processed * 1000L) / durationMillis;
    }

    // DTO для метрик (временно тут)
    public record Metrics(long totalTasks, long activeTasks, long completedTasks, long avgExecutionTime) {}
}
