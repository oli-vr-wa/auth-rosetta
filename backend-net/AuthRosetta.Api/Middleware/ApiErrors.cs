using AuthRosetta.Application.Validation;

namespace AuthRosetta.Api.Middleware;

public static class ApiErrors
{
    public static string MessageFor(int status) => status switch
    {
        400 => "Invalid request",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not found",
        405 => "Method not allowed",
        415 => "Unsupported media type",
        >= 500 => "Internal server error",
        _ => "Request failed"
    };

    /// <summary>
    /// Adds error handling middleware to the application pipeline. 
    /// This middleware captures exceptions and returns appropriate error responses in JSON format.
    /// </summary>
    /// <param name="app">The application builder.</param>
    /// <returns>The application builder with error handling middleware added.</returns>
    public static IApplicationBuilder UseApiErrorHandling(this IApplicationBuilder app)
    {
        app.UseMiddleware<ErrorHandlingMiddleware>();

        app.UseStatusCodePages(async ctx =>
        {
            var response = ctx.HttpContext.Response;
            await response.WriteAsJsonAsync(new { error = MessageFor(response.StatusCode) });
        });

        return app;
    }
}

/// <summary>
/// Middleware that handles exceptions and returns appropriate error responses in JSON format.
/// </summary>
/// <param name="next">The next middleware in the pipeline.</param>
/// <param name="logger">The logger instance.</param>
public sealed class ErrorHandlingMiddleware(RequestDelegate next, ILogger<ErrorHandlingMiddleware> logger)
{
    public async Task InvokeAsync(HttpContext context)
    {
        try
        {
            await next(context);
        }
        catch (OperationCanceledException) when (context.RequestAborted.IsCancellationRequested)
        {
            // Request was canceled by the client, no need to log or return an error response.
        }
        catch (Exception ex) when (!context.Response.HasStarted)
        {
            var (status, message) = ex switch
            {
                RequestValidationException => (StatusCodes.Status400BadRequest, ex.Message),
                BadHttpRequestException => (StatusCodes.Status400BadRequest, ApiErrors.MessageFor(400)),
                _ => (StatusCodes.Status500InternalServerError, ApiErrors.MessageFor(500))
            };

            if (status >= 500)
                logger.LogError(ex, "{Method} {Path} -> {Status}", context.Request.Method, context.Request.Path, status);
            else
                logger.LogWarning("{Method} {Path} -> {Status}: {Reason}", context.Request.Method, context.Request.Path, status, ex.Message);                            

            context.Response.StatusCode = status;
            await context.Response.WriteAsJsonAsync(new { error = message });
        }
    }
}
