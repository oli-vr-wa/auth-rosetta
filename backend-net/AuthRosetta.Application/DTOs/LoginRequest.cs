namespace AuthRosetta.Application.DTOs;

public record LoginRequest(string Email, string Password, string Code);
