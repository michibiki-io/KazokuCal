FROM node:22-bookworm-slim AS frontend-build
ARG BUILD_VERSION=
ARG BUILD_COMMIT=
ARG GITHUB_REPOSITORY=michibiki-io/KazokuCal
ENV BUILD_VERSION=${BUILD_VERSION} \
    BUILD_COMMIT=${BUILD_COMMIT} \
    GITHUB_REPOSITORY=${GITHUB_REPOSITORY}
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26.2-bookworm AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/kazokucal ./cmd/server

FROM debian:13-slim AS runtime
ENV PORT=8080 \
    AUTH_ENABLED=false \
    AUTH_USER_HEADER=X-Forwarded-User \
    AUTH_EMAIL_HEADER=X-Forwarded-Email \
    AUTH_GROUPS_HEADER=X-Forwarded-Groups \
    AUTHORIZED_GROUPS= \
    APP_BASE_PATH= \
    STATIC_DIR=/app/static \
    PDFGEN_SCRIPT=/app/pdfgen/generate_calendar.py \
    HOLIDAY_SCRIPT=/app/pdfgen/list_holidays.py \
    PATH=/opt/venv/bin:$PATH

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates python3 python3-venv fonts-morisawa-bizud-gothic fonts-noto-core fonts-noto-extra \
    && python3 -m venv /opt/venv \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY pdfgen/requirements.txt /app/pdfgen/requirements.txt
RUN /opt/venv/bin/pip install --no-cache-dir -r /app/pdfgen/requirements.txt
COPY pdfgen/ /app/pdfgen/
COPY --from=frontend-build /src/frontend/dist/ /app/static/
COPY --from=backend-build /out/kazokucal /app/server

EXPOSE 8080
CMD ["/app/server"]
