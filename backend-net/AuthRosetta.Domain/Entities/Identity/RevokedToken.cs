namespace AuthRosetta.Domain.Entities.Identity;

public class RevokedToken
{
    public string Jti { get; set; } = default!;
    public DateTime ExpiresAt { get; set; }
}
