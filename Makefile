# Makefile для сборки FocusFrame

# Имя бинарного файла
BINARY=focusframe

# Компилятор
GO=go

# Флаги сборки
BUILD_FLAGS=-o $(BINARY)

# Цель по умолчанию
all: build

# Сборка программы
build:
	$(GO) build $(BUILD_FLAGS)

# Очистка
clean:
	rm -f $(BINARY)

# Установка зависимостей (если нужно)
deps:
	$(GO) mod init focusframe || true  # Инициализация модуля, если его нет
	$(GO) mod tidy
	$(GO) get -u ./...

# Запуск программы
run: build
	./$(BINARY)

.PHONY: all build clean deps run