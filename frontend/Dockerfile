# Node.js 베이스 이미지 사용
FROM node:18-alpine

# 작업 디렉토리 설정
WORKDIR /app

# pnpm 설치
RUN npm install -g pnpm

# 의존성 파일 복사 및 설치
COPY package.json pnpm-lock.yaml* ./
RUN npm install

# 소스 코드 복사
COPY . .

# 환경 변수 설정
ARG REACT_APP_API_URL
ENV REACT_APP_API_URL=$REACT_APP_API_URL

# 애플리케이션 빌드
RUN pnpm run build

# 포트 노출 (애플리케이션이 사용하는 포트에 맞게 수정)
EXPOSE 3000

ENV NODE_ENV=production

# 애플리케이션 실행
CMD ["pnpm", "start"]