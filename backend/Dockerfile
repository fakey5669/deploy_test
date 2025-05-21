# Go 빌드 환경 설정
FROM golang:1.23 AS builder

WORKDIR /app

# 의존성 다운로드 및 캐싱
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 복사 및 빌드
COPY . ./
WORKDIR /app/cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /k8scontrol

# 실행 환경 설정 (더 작은 이미지 사용)
FROM alpine:3.19

# 필요한 CA 인증서 및 시간대 데이터 설치
RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates

# 비루트 사용자 생성
RUN adduser -D -h /app appuser
WORKDIR /app

# 바이너리 복사 및 권한 설정
COPY --from=builder /k8scontrol /app/k8scontrol
RUN chmod +x /app/k8scontrol && \
    chown appuser:appuser /app/k8scontrol

# 환경 변수 설정
ENV APP_ENV=production

# 비루트 사용자로 전환
USER appuser

# 포트 노출
EXPOSE 8080

# 실행
CMD ["/app/k8scontrol"]
