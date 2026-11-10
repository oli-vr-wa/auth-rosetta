using AuthRosetta.Domain.Entities.Identity;
using Microsoft.EntityFrameworkCore;

namespace AuthRosetta.Infrastructure.Data;

public class AuthDbContext(DbContextOptions<AuthDbContext> options) : DbContext(options)
{
	public DbSet<User> Users => Set<User>();
    public DbSet<RevokedToken> RevokedTokens => Set<RevokedToken>();

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        modelBuilder.Entity<User>(entity =>
        {
            entity.HasIndex(u => u.Email).IsUnique();
            entity.Property(u => u.Email).HasMaxLength(256).IsRequired();
            entity.Property(u => u.PasswordHash).IsRequired();
            entity.Property(u => u.TotpSecret).IsRequired();
        });

        modelBuilder.Entity<RevokedToken>().HasKey(rt => rt.Jti);
    }
}
