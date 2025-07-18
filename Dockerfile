FROM golang:latest AS backend
WORKDIR /nytrpg
COPY . .
RUN go mod download
RUN go build -o app

FROM node:latest AS frontend
WORKDIR /nytrpg
COPY . .
RUN npm install
RUN npx tsc
COPY --from=backend /nytrpg/app .
EXPOSE 8080
CMD ["./app"]
