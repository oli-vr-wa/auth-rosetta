using AuthRosetta.Application.DTOs;
using AuthRosetta.Application.Interfaces;
using AuthRosetta.Application.Validation;
using AuthRosetta.Domain.Entities.Identity;
using Microsoft.Extensions.Logging;
using System.Reflection.Metadata;

namespace AuthRosetta.Application.Services;

public class AuthService(
    IUserRepository userRepository,
    IRevokedTokenRepository revokedTokenRepository,
    IPasswordService passwordService,
    ITotpService totpService,
    ITokenService tokenService,
    ILogger<AuthService> logger) : IAuthService
{
    private static string NormalizeEmail(string email) => email.Trim().ToLowerInvariant();

    /// <inheritdoc />
    public async Task<RegisterResult?> RegisterAsync(RegisterRequest request)
    {
        RegisterRules.EnsureValid(request.Email, request.Password);

        var email = NormalizeEmail(request.Email);
        if (await userRepository.GetByEmailAsync(email) is not null) return null;

        var secret = totpService.GenerateSecret();
        var user = new User
        {
            Email = email,
            PasswordHash = passwordService.Hash(request.Password),
            TotpSecret = secret,
        };

        if (!await userRepository.TryAddAsync(user)) return null;

        return new RegisterResult(totpService.BuildUri(email, secret), secret);
    }

    /// <inheritdoc />
    public async Task<bool> ConfirmTwoFactorAuthenticationAsync(ConfirmRequest request)
    {        
        var user = TextRules.IsStorable(NormalizeEmail(request.Email)) ? await userRepository.GetByEmailAsync(NormalizeEmail(request.Email)) : null;
        if (user is null || !totpService.Verify(user.TotpSecret, request.Code)) return false;

        user.TwoFactorEnabled = true;
        await userRepository.SaveChangesAsync();
        return true;
    }

    /// <inheritdoc />
    public async Task<string?> LoginAsync(LoginRequest request)
    {
        var normalizedEmail = NormalizeEmail(request.Email);
        
        var user = TextRules.IsStorable(normalizedEmail) ? await userRepository.GetByEmailAsync(normalizedEmail) : null;

        var passwordOk = user is not null
            ? passwordService.Verify(request.Password, user.PasswordHash)
            : passwordService.VerifyAgainstDummy(request.Password);
        var codeOk = user is not null && totpService.Verify(user.TotpSecret, request.Code);

        if (user is null
            || !passwordOk
            || !user.TwoFactorEnabled
            || !codeOk)
        {
            logger.LogDebug(
                "Login rejected (user found: {UserFound}, password ok: {PasswordOk}, two-factor enabled: {TwoFactorEnabled}, code ok: {CodeOk})",
                user is not null,
                passwordOk,
                user?.TwoFactorEnabled,
                codeOk);

            return null;
        }

        return tokenService.CreateToken(user.Id, user.Email);
    }    

    /// <inheritdoc />
    public Task LogoutAsync(string jti, DateTime expiresAt) => revokedTokenRepository.RevokeAsync(jti, expiresAt);
}
