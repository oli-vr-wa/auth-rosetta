namespace AuthRosetta.Api.Middleware;

public sealed class RequestBodyTooLargeException() : Exception("Request body exceeds the allowed size");
