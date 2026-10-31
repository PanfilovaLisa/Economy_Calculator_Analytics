# Economy_Calculator_Analytics

```bash
cd Economy_Calculator_Analytics
cd backend
go build ./...
go run cmd/main.go

#в другом терминале
cd Economy_Calculator_Analytics
curl -X POST http://localhost:8082/calculate -F "file=@testfile/1deal.csv"
```