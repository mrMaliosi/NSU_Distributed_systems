package ru.nsu.ccfit.malinovskii.crackhash2.config;

import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;
import org.springframework.scheduling.annotation.EnableScheduling;

@Configuration
@EnableScheduling
@Profile("dispatcher")
public class SchedulingConfiguration {
}
