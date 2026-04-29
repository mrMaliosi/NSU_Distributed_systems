//package ru.nsu.ccfit.malinovskii.crackhash2.config;
//
//import jakarta.annotation.PostConstruct;
//import lombok.RequiredArgsConstructor;
//import org.springframework.context.annotation.Configuration;
//import org.springframework.data.mongodb.core.MongoTemplate;
//import org.springframework.data.mongodb.core.index.Index;
//
//@Configuration
//@RequiredArgsConstructor
//public class MongoIndexesConfig {
//
//    private final MongoTemplate mongoTemplate;
//
//    @PostConstruct
//    public void initIndexes() {
//
//        mongoTemplate.indexOps("requests")
//                .ensureIndex(new Index()
//                        .on("hash", org.springframework.data.domain.Sort.Direction.ASC)
//                        .on("maxLength", org.springframework.data.domain.Sort.Direction.ASC)
//                        .on("algorithm", org.springframework.data.domain.Sort.Direction.ASC)
//                        .on("alphabet", org.springframework.data.domain.Sort.Direction.ASC)
//                        .unique());
//    }
//}