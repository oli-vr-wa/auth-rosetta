using AuthRosetta.Application.Interfaces;
using AuthRosetta.Domain.Entities.Identity;
using AuthRosetta.Infrastructure.Data;
using Microsoft.EntityFrameworkCore;

namespace AuthRosetta.Infrastructure.Repositories;

public class RevokedTokenRepository(AuthDbContext dbContext) : IRevokedTokenRepository
{
    /// <inheritdoc />
    public async Task RevokeAsync(string jti, DateTime expiresAt)
    {
        // Housekeeping: Remove expired revoked tokens
        await dbContext.RevokedTokens
            .Where(rt => rt.ExpiresAt <= DateTime.UtcNow)
            .ExecuteDeleteAsync();

        if (!await dbContext.RevokedTokens.AnyAsync(t => t.Jti == jti))
        {
            dbContext.RevokedTokens.Add(new RevokedToken
            {
                Jti = jti,
                ExpiresAt = expiresAt
            });
            await dbContext.SaveChangesAsync();
        }
    }

    /// <inheritdoc />
    public Task<bool> IsRevokedAsync(string jti) =>
        dbContext.RevokedTokens.AnyAsync(t => t.Jti == jti);
}
