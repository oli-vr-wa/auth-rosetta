using AuthRosetta.Application.Interfaces;
using AuthRosetta.Domain.Entities.Identity;
using AuthRosetta.Infrastructure.Data;
using Microsoft.EntityFrameworkCore;

namespace AuthRosetta.Infrastructure.Repositories;

public class UserRepository(AuthDbContext db) : IUserRepository
{
    /// <inheritdoc />
    public void Add(User user) => db.Users.Add(user);

    /// <inheritdoc />
    public Task<User?> GetByEmailAsync(string email) => db.Users.FirstOrDefaultAsync(u => u.Email == email);    

    /// <inheritdoc />
    public Task SaveChangesAsync() => db.SaveChangesAsync();
}
