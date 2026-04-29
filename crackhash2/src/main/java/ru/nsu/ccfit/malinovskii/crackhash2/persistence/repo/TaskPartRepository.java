package ru.nsu.ccfit.malinovskii.crackhash2.persistence.repo;

import org.springframework.data.mongodb.repository.MongoRepository;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskPart;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskStatus;

import java.util.List;

public interface TaskPartRepository extends MongoRepository<TaskPart, String> {

    List<TaskPart> findByRequestId(String requestId);

    long countByRequestId(String requestId);

    List<TaskPart> findByStatus(TaskStatus status);

    List<TaskPart> findByStatusIn(List<TaskStatus> statuses);

    List<TaskPart> findByRequestIdAndStatus(String requestId, TaskStatus status);

    long countByRequestIdAndStatus(String requestId, TaskStatus status);
}