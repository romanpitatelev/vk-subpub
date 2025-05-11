## Описание сервиса
Сервис реализует паттерн Publisher-Subscriber (PubSub) с использованием gRPC. 

## Основные возможности:
- Подписка на события по ключу (топику)
- Публикация событий в топики
- Управление подключенными подписчиками

## Архитектурные паттерны
- Graceful Shutdown
- Dependency Inversion


## Как запустить сервис
```bash
# Terminal 1
make run
```

## Примеры запросов

Подписаться на топик:
```
grpcurl -plaintext -d '{"key": "news"}' localhost:9000 subscription.v1.PubSub/Subscribe
```
Опубликовать сообщение:
```
grpcurl -plaintext -d '{"key": "news", "data": "breaking news!"}' localhost:9000 subscription.v1.PubSub/Publish
```
## Тестирование
Запуск тестов:
```bash
make test
```
- Покрытие составляет больше 60% 
