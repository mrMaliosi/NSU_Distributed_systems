package ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity;

import java.util.List;
import lombok.Data;
import org.springframework.data.mongodb.core.index.CompoundIndex;
import org.springframework.data.mongodb.core.mapping.Document;

@Data
@Document("requests")
@CompoundIndex(
        name = "unique_request_idx",
        def = "{'hash':1,'maxLength':1,'algorithm':1,'alphabet':1}",
        unique = true
)
public class HashRequest {
    private String id;

    private String hash;
    private int maxLength;
    private String algorithm;
    private String alphabet;

    private RequestStatus status;

    private List<String> result;

    private Long createdAt;
    private Long updatedAt;
}