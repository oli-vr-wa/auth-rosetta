namespace AuthRosetta.Application.Validation;

public sealed class RequestValidationException(string message) : Exception(message);
