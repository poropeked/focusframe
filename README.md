# FocusFrame

FocusFrame — это инструмент для захвата и отслеживания определённой области экрана в реальном времени с возможностью создания скриншотов. Программа позволяет выбрать окно, выделить интересующую область внутри него, запустить автоматический захват и сохранять кадры в формате PNG.

Основные возможности:
- Выбор окна или произвольной области экрана.
- Автоматический захват кадров в реальном времени.
- Возможность делать скриншоты вручную или автоматически.
- Режим "Поверх всех" для удобного просмотра выбранной области.
- Минимизация интерфейса в "скрытый режим" для ненавязчивого использования.

## Требования

- **Операционная система**: Linux (с X11)
- **Зависимости**:
  - `xdotool` — для выбора окон.
  - `slop` — для выделения области экрана.
  - `wmctrl` — для управления параметрами окна (например, "Поверх всех").
- **Go**: версия 1.16 или выше для сборки из исходников.

Установите зависимости в Ubuntu/Debian:
```bash
sudo apt update
sudo apt install xdotool slop wmctrl
```
## Установка
1. Клонируйте репозиторий или скачайте исходный код:
```bash
git clone https://github.com/poropeked/focusframe.git
cd focusframe
```
2. Соберите программу с помощью make:
```bash
make build
```
После выполнения команды будет создан бинарный файл focusframe.

## Использование
1. Запустите программу:
```bash
./focusframe
```
Убедитесь, что используется X11 (Wayland не поддерживается).

2. Интерфейс:
    - Выбрать окно: Нажмите, чтобы выбрать окно для захвата.
    - Выделить: Выделите область внутри выбранного окна (кнопка активна после выбора окна).
    - Старт/Стоп: Запустите или остановите захват выделенной области (активна после выделения).
    - Поверх всех: Переключите окно поверх других или верните в обычный режим.
    - Тёмная/Светлая: Смена темы интерфейса.
    - Инфо: Показать/скрыть информацию о статусе, окне и области.
    - Скриншот: Сохранить текущий кадр (активна во время захвата).
    - Скрыть: Скрыть панель инструментов.
3. Скрытый режим:
    - После нажатия "Скрыть" появляется узкая полоса слева (ширина 8 пикселей).
    - Кликните на полосу, чтобы показать кнопки ">" (вернуть панель) и "Скриншот" (сохранить кадр, если захват активен).
    - Кнопки исчезают через 2 секунды бездействия.
4. Сохранение скриншота:
    - Нажмите "Скриншот" (в панели или скрытом режиме) — откроется диалог выбора пути для сохранения файла в формате PNG.
