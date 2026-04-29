package ru.nsu.ccfit.malinovskii.crackhash2.utils;

public class SplitterUtils {
    public static long estimate(int alphabetSize, int maxLength) {
        long total = 0;

        for (int i = 1; i <= maxLength; i++) {
            total += (long) Math.pow(alphabetSize, i);
        }

        return total;
    }
}
