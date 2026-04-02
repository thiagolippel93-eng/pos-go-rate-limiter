# Rate Limiter

Rate Limiter em Go implementado como middleware HTTP, com suporte a limitação por IP e por Token de acesso, utilizando Redis para persistência.

O projeto utiliza o padrão **Strategy** para a camada de persistência. A interface `StorageStrategy` (`storage/storage.go`) define os métodos necessários, permitindo substituir o Redis por outro backend alterando apenas a implementação, sem tocar na lógica de negócio ou no middleware.

### Precedência Token > IP

Quando a requisição possui o header `API_KEY`, o limite do token é aplicado e o limite por IP é ignorado. Se um token específico tiver configuração própria (`TOKEN_<NOME>_*`), ela prevalece sobre os padrões de token.

## Executando o projeto

### 1. Iniciar o Redis

```bash
docker compose up -d
```

### 2. Rodar o servidor

```bash
go run main.go
```

O servidor inicia na porta configurada (padrão `8080`).

### 3. Testar

```bash
# Requisição limitada por IP
curl http://localhost:8080/

# Requisição limitada por Token
curl -H "API_KEY: abc123" http://localhost:8080/
```

Ao exceder o limite, a resposta será:

```
HTTP 429 Too Many Requests

you have reached the maximum number of requests or actions allowed within a certain time frame
```

### 4. Executar os testes

```bash
# Executar todos os testes
go test ./... -v

# Executar apenas testes do limiter
go test ./limiter/... -v

# Executar apenas testes do middleware
go test ./middleware/... -v
```