package ru.nsu.ccfit.malinovskii.crackhash2.services;

import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import ru.nsu.ccfit.malinovskii.crackhash2.persistence.entity.TaskMessage;

import java.security.MessageDigest;

@Service
@Slf4j
public class BruteForceService {

    public String crack(TaskMessage task) {
        for (long i = task.getRangeStart(); i < task.getRangeEnd(); i++) {

            String candidate = indexToWord(i, task.getAlphabet(), task.getMaxLength());

            String hash = hash(candidate, task.getAlgorithm());

            if (hash.equalsIgnoreCase(task.getHash())) {
                log.info("FOUND: {}", candidate);
                return candidate;
            }
        }
        return null;
    }

    /**
     * index → слово
     */
    private String indexToWord(long index, String alphabet, int maxLength) {
        int base = alphabet.length();

        StringBuilder word = new StringBuilder();

        while (index > 0) {
            int rem = (int)(index % base);
            word.append(alphabet.charAt(rem));
            index /= base;
        }

        return word.reverse().toString();
    }

    /**
     * Хэширование
     */
    private String hash(String input, String algorithm) {
        try {
            MessageDigest md = MessageDigest.getInstance(algorithm);
            byte[] digest = md.digest(input.getBytes());

            StringBuilder hex = new StringBuilder();
            for (byte b : digest) {
                hex.append(String.format("%02x", b));
            }

            return hex.toString();
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }
}