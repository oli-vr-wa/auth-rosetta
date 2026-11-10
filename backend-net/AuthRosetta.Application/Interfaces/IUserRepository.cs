using AuthRosetta.Domain.Entities.Identity;

namespace AuthRosetta.Application.Interfaces;

public interface IUserRepository
{
    /// <summary>
    /// Gets a user by their email address.
    /// </summary>
    /// <param name="email">The email address of the user.</param>
    /// <returns>The user with the specified email address, or null if no such user exists.</returns>
    Task<User?> GetByEmailAsync(string email);

    /// <summary>
    /// Adds a new user to the repository.
    /// </summary>
    /// <param name="user">The user to add.</param>
    void Add(User user);

    /// <summary>
    /// Saves changes made to the repository.
    /// </summary>
    /// <returns>A task that represents the asynchronous operation.</returns>
    Task SaveChangesAsync();
}
