using AuthRosetta.Application.Interfaces;
using AuthRosetta.Domain.Entities.Identity;
using AuthRosetta.Infrastructure.Data;
using Microsoft.EntityFrameworkCore;
using Npgsql;

namespace AuthRosetta.Infrastructure.Repositories;

public class UserRepository(AuthDbContext db) : IUserRepository
{
    /// <inheritdoc />
    public Task<User?> GetByEmailAsync(string email) => db.Users.FirstOrDefaultAsync(u => u.Email == email);    

    /// <inheritdoc />
    public Task SaveChangesAsync() => db.SaveChangesAsync();

    /// <inheritdoc />
    public async Task<bool> TryAddAsync(User user)
    {
        db.Users.Add(user);
        try
        {
            await db.SaveChangesAsync();
            return true;
        }
        catch (DbUpdateException ex) when (ex.InnerException is PostgresException
        {
            SqlState: PostgresErrorCodes.UniqueViolation
        })
        {
            db.Entry(user).State = EntityState.Detached;
            return false;
        }
    }
}
