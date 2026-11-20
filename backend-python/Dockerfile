FROM python:3.14-slim

# No .pyc files in the image; flush logs immediately so `docker compose logs` is live.
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

# Dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app

RUN useradd --system --no-create-home appuser
USER appuser

EXPOSE 5003
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "5003"]