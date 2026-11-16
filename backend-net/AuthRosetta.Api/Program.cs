using AuthRosetta.Application.Interfaces;
using AuthRosetta.Infrastructure.Services;
using AuthRosetta.Infrastructure.Data;
using Microsoft.EntityFrameworkCore;
using Npgsql;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.IdentityModel.Tokens;
using System.Text;
using AuthRosetta.Api.Endpoints;
using AuthRosetta.Infrastructure.Repositories;
using AuthRosetta.Application.Services;
using Microsoft.AspNetCore.Authorization;
using AuthRosetta.Api.Helpers;
using AuthRosetta.Api.Configuration;
using AuthRosetta.Api.Middleware;

if (Environment.GetEnvironmentVariable("DOTNET_RUNNING_IN_CONTAINER") != "true")
    DotNetEnv.Env.TraversePath().Load(); // Load environment variables from .env file

var config = AppConfig.FromEnvironment(); // Load configuration from environment variables

var builder = WebApplication.CreateBuilder(args);

// Throw on bad request in every environment, so we can handle it in our middleware and handle it same way everywhere.
builder.Services.Configure<RouteHandlerOptions>(options => options.ThrowOnBadRequest = true);

builder.Services.ConfigureHttpJsonOptions(options =>
{
    options.SerializerOptions.RespectNullableAnnotations = true;            // Explicit null is rejected.
    options.SerializerOptions.RespectRequiredConstructorParameters = true;  // A missing field is rejected.
});

var connectionString = new NpgsqlConnectionStringBuilder(builder.Configuration.GetConnectionString("DefaultConnection"))
{
    Username = config.DbUser,
    Password = config.DbPassword
}.ConnectionString;
builder.Services.AddDbContext<AuthDbContext>(o => o.UseNpgsql(connectionString));

var jwt = new JwtOptions
{
    Key = Environment.GetEnvironmentVariable("JWT_KEY")
        ?? throw new InvalidOperationException("JWT_KEY environment variable is not set."),
    Issuer = builder.Configuration["Jwt:Issuer"]!,
    Audience = builder.Configuration["Jwt:Audience"]!,
    ExpiryMinutes = builder.Configuration.GetValue<int>("Jwt:ExpiryMinutes")
};
builder.Services.AddSingleton(jwt);

// Add services to the container.
builder.Services.AddSingleton<IPasswordService, PasswordService>();
builder.Services.AddSingleton<ITotpService, TotpService>();
builder.Services.AddSingleton<ITokenService, TokenService>();
builder.Services.AddScoped<IAuthService, AuthService>();
builder.Services.AddScoped<IUserRepository, UserRepository>();
builder.Services.AddScoped<IRevokedTokenRepository, RevokedTokenRepository>();
builder.Services.AddOpenApi();

builder.Services.AddCors(options => options.AddPolicy("Frontend", policy =>
    policy.WithOrigins("http://localhost:5173")
    .AllowAnyHeader()
    .AllowAnyMethod()
    .AllowCredentials()));

builder.Services
    .AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
    .AddJwtBearer(o =>
    {
        o.MapInboundClaims = false;
        o.TokenValidationParameters = new TokenValidationParameters
        {
            ValidateIssuer = true,
            ValidIssuer = jwt.Issuer,
            ValidateAudience = true,
            ValidAudience = jwt.Audience,
            ValidateIssuerSigningKey = true,
            IssuerSigningKey = new SymmetricSecurityKey(Encoding.UTF8.GetBytes(jwt.Key)),
            ValidAlgorithms = [SecurityAlgorithms.HmacSha256],
            ValidateLifetime = true,
            ClockSkew = TimeSpan.FromSeconds(30)
        };
        o.Events = new JwtBearerEvents
        {
            OnMessageReceived = context =>
            {
                context.Token = context.Request.Cookies[AuthCookie.Name];
                return Task.CompletedTask;
            },
            OnAuthenticationFailed = context =>
            {
                context.HttpContext.RequestServices.GetRequiredService<ILoggerFactory>()
                    .CreateLogger("AuthRosetta.Api.Jwt")
                    .LogDebug("Token rejected: {Reason}", context.Exception.Message);
                return Task.CompletedTask;
            },
            OnTokenValidated = async context =>
            {
                var jti = context.Principal?.FindFirst("jti");
                var repo = context.HttpContext.RequestServices.GetRequiredService<IRevokedTokenRepository>();

                if (jti is null || await repo.IsRevokedAsync(jti.Value))
                {
                    context.HttpContext.RequestServices.GetRequiredService<ILoggerFactory>()
                        .CreateLogger("AuthRosetta.Api.Jwt")
                        .LogDebug("Token rejected: revoked or missing jti");
                    context.Fail("Token has been revoked");
                }
            }
        };
    });

builder.Services.AddAuthorization(options =>
    options.FallbackPolicy = new AuthorizationPolicyBuilder().RequireAuthenticatedUser().Build());

builder.Services.AddCors(options => options.AddPolicy("Frontend", policy => policy
    .WithOrigins(config.FrontendOrigin)
    .WithMethods("GET", "POST", "OPTIONS")
    .WithHeaders("Content-Type")
    .AllowCredentials()));

var app = builder.Build();

// Configure the HTTP request pipeline.
if (app.Environment.IsDevelopment())
{
    app.MapOpenApi();
}

app.UseHttpsRedirection();
app.UseCors("Frontend");
app.UseApiErrorHandling();  // Add error handling middleware to the pipeline
app.UseAuthentication();
app.UseAuthorization();

app.MapAuthEndpoints();

app.Run();
