namespace AuthRosetta.Api.Middleware;

/// <summary>
/// Middleware to enforce a maximum request body size. Throws a RequestBodyTooLargeException if the request body exceeds the limit.
/// </summary>
/// <param name="next">The next middleware in the pipeline.</param>
public sealed class BodySizeLimitMiddleware(RequestDelegate next)
{
    public const long MaxBytes = 8 * 1024; // 8 KB max request body size to enforce API contract

    public async Task InvokeAsync(HttpContext context)
    {
        if (context.Request.ContentLength > MaxBytes)
            throw new RequestBodyTooLargeException();

        context.Request.Body = new LimitedReadStream(context.Request.Body, MaxBytes);
        await next(context);
    }
}

/// <summary>
/// Stream wrapper that limits the number of bytes that can be read from the underlying stream. 
/// Throws a RequestBodyTooLargeException if the limit is exceeded.
/// </summary>
/// <param name="inner">The underlying stream to read from.</param>
/// <param name="maxBytes">The maximum number of bytes allowed to be read from the stream.</param>
internal sealed class LimitedReadStream(Stream inner, long maxBytes) : Stream
{
    private long _total;

    public override bool CanRead => true;
    public override bool CanSeek => false;
    public override bool CanWrite => false;
    public override long Length => throw new NotSupportedException();
    public override long Position
    {
        get => throw new NotSupportedException();
        set => throw new NotSupportedException();
    }

    public override async ValueTask<int> ReadAsync(Memory<byte> buffer, CancellationToken cancellationToken = default)
    {
        var read = await inner.ReadAsync(buffer, cancellationToken);
        Count(read);
        return read;
    }

    public override Task<int> ReadAsync(byte[] buffer, int offset, int count, CancellationToken cancellationToken = default)
        => ReadAsync(buffer.AsMemory(offset, count), cancellationToken).AsTask();

    public override int Read(byte[] buffer, int offset, int count)
    {
        var read = inner.Read(buffer, offset, count);
        Count(read);
        return read;
    }

    /// <summary>
    /// Increments the total number of bytes read and checks if it exceeds the maximum allowed.
    /// </summary>
    /// <param name="read">The number of bytes read from the stream.</param>
    /// <exception cref="RequestBodyTooLargeException">Thrown if the total number of bytes read exceeds the maximum allowed.</exception>
    private void Count(int read)
    {
        _total += read;
        if (_total > maxBytes)
            throw new RequestBodyTooLargeException();
    }

    public override void Flush() { }
    public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();
    public override void SetLength(long value) => throw new NotSupportedException();
    public override void Write(byte[] buffer, int offset, int count) => throw new NotSupportedException();
}