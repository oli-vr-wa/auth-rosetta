using System;

namespace AuthRosetta.Domain.Entities.Identity;

public class User
{
    public Guid Id { get; set; } = Guid.NewGuid();

    public string Email { get; set; } = default!;
    public string PasswordHash { get; set; } = default!;

    public string TotpSecret { get; set; } = default!;
    public bool TwoFactorEnabled { get; set; }

    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;
}