package com.authrosetta.java.api;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController 
public class HealthController {
    
    public record HealthResponse(String status) {}

    @GetMapping("/health")
    public HealthResponse health() {
        return new HealthResponse("ok");
    }
}
