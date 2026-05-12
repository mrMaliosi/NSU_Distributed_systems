FROM eclipse-temurin:21-jdk-alpine AS build
WORKDIR /src
COPY gradle gradle
COPY gradlew build.gradle settings.gradle ./
COPY src src
RUN chmod +x gradlew && ./gradlew bootJar --no-daemon

FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=build /src/build/libs/*.jar app.jar
EXPOSE 8081
ENTRYPOINT ["java", "-jar", "app.jar"]
