package com.authrosetta.java;

import java.util.Map;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

import com.authrosetta.java.config.AppConfig;

@SpringBootApplication
public class AuthRosettaJavaApplication {

	public static void main(String[] args) {
		AppConfig config = AppConfig.loadDefault();
		SpringApplication app = new SpringApplication(AuthRosettaJavaApplication.class);
		app.setDefaultProperties(Map.of("logging.level.root", config.springLogLevel()));
		app.run(args);
	}

}
