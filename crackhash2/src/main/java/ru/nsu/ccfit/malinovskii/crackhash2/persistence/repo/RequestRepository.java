package ru.nsu.ccfit.malinovskii.crackhash2.persistence.repo;

import org.springframework.data.mongodb.repository.MongoRepository;
import org.springframework.stereotype.Repository;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.HashRequest;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.RequestStatus;

import java.util.List;
import java.util.Optional;

@Repository
public interface RequestRepository extends MongoRepository<HashRequest, String> {

    /**
     * Для идемпотентности
     */
    Optional<HashRequest> findByHashAndMaxLengthAndAlgorithmAndAlphabet(
            String hash,
            int maxLength,
            String algorithm,
            String alphabet
    );

    List<HashRequest> findByStatus(RequestStatus status);

    long countByStatus(RequestStatus status);

    List<HashRequest> findByStatusIn(List<RequestStatus> statuses);

    List<HashRequest> findByStatusNot(RequestStatus status);

}