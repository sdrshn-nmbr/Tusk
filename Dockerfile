FROM python:3.12-slim

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE 1
ENV PYTHONUNBUFFERED 1

WORKDIR /app

# Install dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy the rest of the application code
COPY . .

# Expose the port the app runs on
EXPOSE 8000

# Start the application with Gunicorn
CMD ["gunicorn", "-w", "${GUNICORN_WORKERS:-4}", "-k", "uvicorn.workers.UvicornWorker", "main:app"]
