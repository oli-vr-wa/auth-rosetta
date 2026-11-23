package com.authrosetta.java;

import java.nio.charset.StandardCharsets;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Primary;

import com.authrosetta.java.config.AppConfig;

@SpringBootTest
class AuthRosettaJavaApplicationTests {

	@TestConfiguration 
	static class TestConfig {
		@Bean 
		@Primary 
		AppConfig testAppConfig() {
			return new AppConfig(
				"localhost",
				5432,
				"auth_java",
				"testuser",
				"testpass",
				"0123456789abcdef0123456789abcdef".getBytes(StandardCharsets.UTF_8),
				"http://localhost:5173",
				"INFO"
			);
		}
	}

	@Test
	void contextLoads() {
	}

}
