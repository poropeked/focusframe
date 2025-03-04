package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

// Указываем, что ResourceIconPng существует (определён в icon.go)
var _ fyne.Resource = resourceIconPng

type FocusFrame struct {
	app                 fyne.App
	window              fyne.Window
	image               *canvas.Image
	infoLabel           *widget.Label
	selectWinBtn        *widget.Button
	selectAreaBtn       *widget.Button
	toggleBtn           *widget.Button
	themeBtn            *widget.Button
	onTopBtn            *widget.Button
	hideBtn             *widget.Button    // Кнопка скрытия панели
	showBtn             *widget.Button    // Кнопка показа панели
	screenshotBtn       *widget.Button    // Кнопка скриншота в панели
	screenshotHiddenBtn *widget.Button    // Кнопка скриншота в скрытом режиме
	clickArea           *widget.Button    // Невидимая кнопка для клика слева
	toolbarScroll       *container.Scroll // Панель кнопок с прокруткой
	infoBtn             *widget.Button    // Кнопка "Инфо"
	infoVisible         bool              // Состояние видимости информации
	capturing           bool
	onTop               bool
	isDarkTheme         bool
	toolbarVisible      bool // Состояние видимости панели
	windowID            xproto.Window
	region              map[string]int
	xconn               *xgb.Conn
	timer               *time.Ticker
	stopChan            chan struct{}
	windowName          string
	areaInfo            string // Информация об области
	captureStatus       string // Статус захвата
	appWindowID         string // ID окна приложения для X11
}

func NewFocusFrame() *FocusFrame {
	a := app.NewWithID("focusframe") // Уникальный ID для приложения
	a.SetIcon(resourceIconPng)       // Устанавливаем иконку
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("FocusFrame") // Имя окна соответствует названию программы
	w.SetFullScreen(false)
	w.Resize(fyne.NewSize(600, 400))

	xconn, err := xgb.NewConn()
	if err != nil {
		panic(fmt.Sprintf("Ошибка подключения к X11: %v", err))
	}

	ff := &FocusFrame{
		app:            a,
		window:         w,
		xconn:          xconn,
		region:         make(map[string]int),
		stopChan:       make(chan struct{}),
		capturing:      false,
		infoVisible:    false,
		onTop:          false,
		isDarkTheme:    true,
		toolbarVisible: true,
		captureStatus:  "Готов к работе",
	}

	// Асинхронно получаем ID окна приложения с использованием --class
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(1 * time.Second)
			out, err := exec.Command("xdotool", "search", "--onlyvisible", "--class", "focusframe").Output()
			if err == nil && len(out) > 0 {
				ff.appWindowID = strings.TrimSpace(string(out))
				break
			}
		}
	}()

	ff.setupUI()
	return ff
}

type fixedWidthLayout struct {
	width float32
}

func (l *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.width, 0) // Фиксированная ширина, высота растягивается
}

func (l *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, obj := range objects {
		obj.Resize(fyne.NewSize(l.width, size.Height)) // Растягиваем по высоте контейнера
		obj.Move(fyne.NewPos(0, 0))
	}
}

func (ff *FocusFrame) setupUI() {
	ff.selectWinBtn = widget.NewButtonWithIcon("Выбрать окно", theme.ViewFullScreenIcon(), ff.selectWindow)
	ff.selectAreaBtn = widget.NewButtonWithIcon("Выделить", theme.ContentCopyIcon(), ff.selectArea)
	ff.selectAreaBtn.Disable()
	ff.toggleBtn = widget.NewButtonWithIcon("Старт", theme.MediaRecordIcon(), ff.toggleCapture)
	ff.toggleBtn.Disable()
	ff.onTopBtn = widget.NewButtonWithIcon("Поверх всех", theme.NavigateNextIcon(), ff.toggleOnTop)
	ff.themeBtn = widget.NewButtonWithIcon("Тёмная", theme.ColorChromaticIcon(), ff.toggleTheme)
	ff.infoBtn = widget.NewButtonWithIcon("Инфо", theme.InfoIcon(), ff.toggleInfo)
	ff.screenshotBtn = widget.NewButtonWithIcon("Скриншот", theme.DocumentSaveIcon(), ff.saveScreenshotWithDialog)
	ff.screenshotBtn.Disable() // Изначально неактивна
	ff.hideBtn = widget.NewButtonWithIcon("Скрыть", theme.VisibilityOffIcon(), ff.hideToolbar)

	// Вертикальная панель кнопок
	toolbar := container.NewVBox(
		ff.selectWinBtn,
		ff.selectAreaBtn,
		ff.toggleBtn,
		ff.onTopBtn,
		ff.themeBtn,
		ff.infoBtn,
		ff.screenshotBtn, // Кнопка "Скриншот" выше "Скрыть"
		ff.hideBtn,
	)
	ff.toolbarScroll = container.NewVScroll(toolbar)

	// Кнопка возврата панели (малозаметная)
	ff.showBtn = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), ff.showToolbar)
	ff.showBtn.Importance = widget.LowImportance
	ff.showBtn.Hide()

	// Кнопка скриншота в скрытом режиме
	ff.screenshotHiddenBtn = widget.NewButtonWithIcon("", theme.DocumentSaveIcon(), ff.saveScreenshotWithDialog)
	ff.screenshotHiddenBtn.Importance = widget.LowImportance
	ff.screenshotHiddenBtn.Hide()

	// Невидимая кнопка для клика слева от изображения (на всю высоту)
	ff.clickArea = widget.NewButton("", func() {
		if !ff.toolbarVisible {
			ff.showBtn.Show()
			if ff.capturing {
				ff.screenshotHiddenBtn.Show()
			}
		}
	})
	ff.clickArea.Importance = widget.LowImportance
	ff.clickArea.Hide()

	ff.image = canvas.NewImageFromImage(nil)
	ff.image.FillMode = canvas.ImageFillContain

	ff.infoLabel = widget.NewLabel(ff.captureStatus)
	ff.infoLabel.Hide()

	// Начальная компоновка
	ff.updateContent()

	// Скрытие кнопок "Показать" и "Скриншот" через таймер
	go func() {
		for range time.Tick(1 * time.Second) {
			if !ff.toolbarVisible && ff.showBtn.Visible() {
				time.Sleep(2 * time.Second)
				if !ff.toolbarVisible {
					ff.showBtn.Hide()
					ff.screenshotHiddenBtn.Hide()
				}
			}
		}
	}()

	// Устанавливаем начальный размер окна
	ff.window.Resize(fyne.NewSize(600, 400))
}

func (ff *FocusFrame) updateContent() {
	var content *fyne.Container
	imageContainer := container.NewMax(ff.image)

	if ff.toolbarVisible {
		// Панель видима: слева кнопки, справа изображение и информация
		content = container.NewBorder(
			nil,              // Верх
			ff.infoLabel,     // Низ
			ff.toolbarScroll, // Слева
			nil,              // Справа
			imageContainer,   // Центр
		)
	} else {
		// Панель скрыта: слева область клика (на всю высоту), справа кнопки "Показать" и "Скриншот" вертикально и изображение
		clickAreaContainer := container.New(&fixedWidthLayout{width: 8}, ff.clickArea) // Уменьшена ширина до 8 пикселей
		leftButtons := container.NewVBox(ff.showBtn, ff.screenshotHiddenBtn)
		leftContainer := container.NewHBox(clickAreaContainer, leftButtons)
		content = container.NewBorder(
			nil,            // Верх
			ff.infoLabel,   // Низ
			leftContainer,  // Слева
			nil,            // Справа
			imageContainer, // Центр
		)
		ff.clickArea.Show()
	}

	ff.window.SetContent(content)
}

func (ff *FocusFrame) hideToolbar() {
	ff.toolbarVisible = false
	ff.showBtn.Show() // Кнопка ">" появляется при скрытии панели
	if ff.capturing {
		ff.screenshotHiddenBtn.Show() // Кнопка "Скриншот" появляется только если захват активен
	} else {
		ff.screenshotHiddenBtn.Hide()
	}
	ff.updateContent()
}

func (ff *FocusFrame) showToolbar() {
	ff.toolbarVisible = true
	ff.showBtn.Hide()
	ff.screenshotHiddenBtn.Hide()
	ff.clickArea.Hide()
	ff.updateContent()
}

func (ff *FocusFrame) updateInfoLabel() {
	info := ff.captureStatus
	if ff.windowName != "" {
		info += fmt.Sprintf("\nОкно: %s (ID: %d)", ff.windowName, ff.windowID)
	}
	if ff.areaInfo != "" {
		info += "\n" + ff.areaInfo
	}
	if ff.infoVisible {
		ff.infoLabel.SetText(info)
	} else {
		ff.infoLabel.SetText("")
	}
}

func (ff *FocusFrame) saveScreenshotWithDialog() {
	if ff.image.Image == nil {
		ff.captureStatus = "Ошибка: нет изображения для сохранения"
		ff.updateInfoLabel()
		return
	}

	// Предлагаем путь для сохранения через диалоговое окно
	dialog.ShowFileSave(func(uri fyne.URIWriteCloser, err error) {
		if err != nil {
			ff.captureStatus = fmt.Sprintf("Ошибка выбора файла: %v", err)
			ff.updateInfoLabel()
			return
		}
		if uri == nil {
			// Пользователь отменил выбор
			return
		}

		file := uri
		defer file.Close()

		err = png.Encode(file, ff.image.Image)
		if err != nil {
			ff.captureStatus = fmt.Sprintf("Ошибка сохранения скриншота: %v", err)
			ff.updateInfoLabel()
			return
		}

		ff.captureStatus = fmt.Sprintf("Скриншот сохранён: %s", file.URI().Path())
		ff.updateInfoLabel()
	}, ff.window)
}

func (ff *FocusFrame) selectWindow() {
	ff.window.Hide()

	cmd := exec.Command("xdotool", "selectwindow")
	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		ff.captureStatus = "Ошибка выбора окна или отмена (Esc)"
		ff.updateInfoLabel()
		ff.window.Show()
		// Восстанавливаем состояние "поверх всех", если оно было активно
		if ff.onTop && ff.appWindowID != "" {
			go ff.restoreOnTop()
		}
		return
	}

	windowIDStr := strings.TrimSpace(string(out))
	windowID, _ := strconv.Atoi(windowIDStr)
	ff.windowID = xproto.Window(windowID)

	name, err := exec.Command("xdotool", "getwindowname", strconv.Itoa(int(ff.windowID))).Output()
	if err != nil {
		ff.captureStatus = "Ошибка получения имени окна"
	} else {
		ff.windowName = strings.TrimSpace(string(name))
		ff.captureStatus = "Готов к работе"
	}
	ff.updateInfoLabel()

	ff.selectWinBtn.Text = "Выбрать окно *"
	ff.selectWinBtn.Refresh()
	ff.selectAreaBtn.Enable() // Активируем кнопку "Выделить" после выбора окна
	ff.window.Show()
	// Восстанавливаем состояние "поверх всех", если оно было активно
	if ff.onTop && ff.appWindowID != "" {
		go ff.restoreOnTop()
	}
}

func (ff *FocusFrame) selectArea() {
	if ff.windowID == 0 {
		ff.captureStatus = "Сначала выберите окно!"
		ff.updateInfoLabel()
		return
	}

	ff.window.Hide()

	cmd := exec.Command("slop", "-f", "%x %y %w %h")
	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		ff.captureStatus = "Ошибка выделения области или отмена (Esc)"
		ff.updateInfoLabel()
		ff.window.Show()
		// Восстанавливаем состояние "поверх всех", если оно было активно
		if ff.onTop && ff.appWindowID != "" {
			go ff.restoreOnTop()
		}
		return
	}

	fields := strings.Fields(string(out))
	if len(fields) != 4 {
		ff.captureStatus = "Некорректные данные от slop"
		ff.updateInfoLabel()
		ff.window.Show()
		// Восстанавливаем состояние "поверх всех", если оно было активно
		if ff.onTop && ff.appWindowID != "" {
			go ff.restoreOnTop()
		}
		return
	}

	absX, _ := strconv.Atoi(fields[0])
	absY, _ := strconv.Atoi(fields[1])
	width, _ := strconv.Atoi(fields[2])
	height, _ := strconv.Atoi(fields[3])

	winInfo, _ := exec.Command("xwininfo", "-id", strconv.Itoa(int(ff.windowID))).Output()
	windowX, windowY := 0, 0
	for _, line := range strings.Split(string(winInfo), "\n") {
		if strings.Contains(line, "Absolute upper-left X:") {
			fmt.Sscanf(line, "  Absolute upper-left X: %d", &windowX)
		}
		if strings.Contains(line, "Absolute upper-left Y:") {
			fmt.Sscanf(line, "  Absolute upper-left Y: %d", &windowY)
		}
	}

	ff.region["left"] = absX - windowX
	ff.region["top"] = absY - windowY
	ff.region["width"] = width
	ff.region["height"] = height

	ff.areaInfo = fmt.Sprintf("Область: %dx%d (x:%d, y:%d)", width, height, ff.region["left"], ff.region["top"])
	ff.captureStatus = "Готов к работе"
	ff.updateInfoLabel()
	ff.selectAreaBtn.Text = "Выделить *"
	ff.selectAreaBtn.Refresh()
	ff.toggleBtn.Enable() // Активируем кнопку "Старт" после выделения области
	ff.window.Show()
	// Восстанавливаем состояние "поверх всех", если оно было активно
	if ff.onTop && ff.appWindowID != "" {
		go ff.restoreOnTop()
	}
}

func (ff *FocusFrame) toggleCapture() {
	if ff.capturing {
		ff.capturing = false
		ff.timer.Stop()
		ff.stopChan <- struct{}{}
		ff.toggleBtn.Text = "Старт"
		ff.toggleBtn.Icon = theme.MediaRecordIcon()
		ff.captureStatus = "Захват остановлен"
		ff.screenshotBtn.Disable()
		ff.screenshotHiddenBtn.Disable()
	} else {
		if len(ff.region) == 0 {
			ff.captureStatus = "Сначала выделите область!"
			ff.updateInfoLabel()
			return
		}
		ff.capturing = true
		ff.timer = time.NewTicker(200 * time.Millisecond)
		ff.toggleBtn.Text = "Стоп"
		ff.toggleBtn.Icon = theme.MediaStopIcon()
		ff.captureStatus = "Захват запущен"
		ff.screenshotBtn.Enable()
		ff.screenshotHiddenBtn.Enable()
		go ff.updateImageLoop()
	}
	ff.updateInfoLabel()
	ff.toggleBtn.Refresh()
}

func (ff *FocusFrame) toggleOnTop() {
	ff.onTop = !ff.onTop
	if ff.onTop {
		go func() {
			if ff.appWindowID != "" {
				err := exec.Command("wmctrl", "-i", "-r", ff.appWindowID, "-b", "add,above").Run()
				if err != nil {
					ff.captureStatus = "Ошибка установки 'Поверх всех': " + err.Error()
				} else {
					err = exec.Command("xdotool", "windowactivate", ff.appWindowID).Run()
					if err != nil {
						ff.captureStatus = "Ошибка активации окна: " + err.Error()
					} else {
						ff.captureStatus = "Установлено 'Поверх всех'"
					}
				}
				ff.updateInfoLabel()
			} else {
				ff.captureStatus = "ID окна не определён для 'Поверх всех'"
				ff.updateInfoLabel()
			}
		}()
		ff.onTopBtn.Text = "Поверх всех"
		ff.onTopBtn.Icon = theme.NavigateNextIcon()
	} else {
		go func() {
			if ff.appWindowID != "" {
				err := exec.Command("wmctrl", "-i", "-r", ff.appWindowID, "-b", "remove,above").Run()
				if err != nil {
					ff.captureStatus = "Ошибка снятия 'Поверх всех': " + err.Error()
				} else {
					ff.captureStatus = "Установлено 'Обычное'"
				}
				ff.updateInfoLabel()
			} else {
				ff.captureStatus = "ID окна не определён для снятия 'Поверх всех'"
				ff.updateInfoLabel()
			}
		}()
		ff.onTopBtn.Text = "Обычное"
		ff.onTopBtn.Icon = theme.NavigateBackIcon()
	}
	ff.onTopBtn.Refresh()
}

// Новая функция для восстановления состояния "поверх всех"
func (ff *FocusFrame) restoreOnTop() {
	if ff.appWindowID != "" {
		err := exec.Command("wmctrl", "-i", "-r", ff.appWindowID, "-b", "add,above").Run()
		if err != nil {
			ff.captureStatus = "Ошибка восстановления 'Поверх всех': " + err.Error()
		} else {
			err = exec.Command("xdotool", "windowactivate", ff.appWindowID).Run()
			if err != nil {
				ff.captureStatus = "Ошибка активации окна: " + err.Error()
			} else {
				ff.captureStatus = "Установлено 'Поверх всех'"
			}
		}
		ff.updateInfoLabel()
	}
}

func (ff *FocusFrame) toggleTheme() {
	if ff.isDarkTheme {
		ff.app.Settings().SetTheme(theme.LightTheme())
		ff.themeBtn.Text = "Светлая"
		ff.themeBtn.Icon = theme.ColorChromaticIcon()
		ff.isDarkTheme = false
		ff.captureStatus = "Переключено на светлую тему"
	} else {
		ff.app.Settings().SetTheme(theme.DarkTheme())
		ff.themeBtn.Text = "Тёмная"
		ff.themeBtn.Icon = theme.ColorChromaticIcon()
		ff.isDarkTheme = true
		ff.captureStatus = "Переключено на тёмную тему"
	}
	ff.updateInfoLabel()
	ff.themeBtn.Refresh()
	ff.window.Content().Refresh()
}

func (ff *FocusFrame) toggleInfo() {
	ff.infoVisible = !ff.infoVisible
	if ff.infoVisible {
		ff.infoLabel.Show()
	} else {
		ff.infoLabel.Hide()
	}
	ff.updateInfoLabel()
}

func (ff *FocusFrame) updateImageLoop() {
	for {
		select {
		case <-ff.timer.C:
			ff.updateImage()
		case <-ff.stopChan:
			return
		}
	}
}

func (ff *FocusFrame) updateImage() {
	if !ff.capturing || ff.windowID == 0 || len(ff.region) == 0 {
		return
	}

	attrs, err := xproto.GetWindowAttributes(ff.xconn, ff.windowID).Reply()
	if err != nil || attrs.MapState != xproto.MapStateViewable {
		ff.captureStatus = "Ошибка: окно не видимо для захвата"
		ff.updateInfoLabel()
		return
	}

	geom, err := xproto.GetGeometry(ff.xconn, xproto.Drawable(ff.windowID)).Reply()
	if err != nil {
		ff.captureStatus = fmt.Sprintf("Ошибка геометрии окна: %v", err)
		ff.updateInfoLabel()
		return
	}

	screenRegion := map[string]int{
		"top":    ff.region["top"],
		"left":   ff.region["left"],
		"width":  min(ff.region["width"], int(geom.Width)-ff.region["left"]),
		"height": min(ff.region["height"], int(geom.Height)-ff.region["top"]),
	}

	if screenRegion["width"] <= 0 || screenRegion["height"] <= 0 {
		ff.captureStatus = "Ошибка: область вне границ окна"
		ff.updateInfoLabel()
		return
	}

	img, err := xproto.GetImage(ff.xconn, xproto.ImageFormatZPixmap, xproto.Drawable(ff.windowID),
		int16(screenRegion["left"]), int16(screenRegion["top"]),
		uint16(screenRegion["width"]), uint16(screenRegion["height"]), 0xffffffff).Reply()
	if err != nil {
		ff.captureStatus = fmt.Sprintf("Ошибка захвата: %v", err)
		ff.updateInfoLabel()
		return
	}

	data := img.Data
	rgbData := make([]uint8, screenRegion["width"]*screenRegion["height"]*3)
	for i := 0; i < len(data); i += 4 {
		b, g, r := data[i], data[i+1], data[i+2] // BGRA -> RGB
		idx := (i / 4) * 3
		rgbData[idx] = r
		rgbData[idx+1] = g
		rgbData[idx+2] = b
	}

	rgba := image.NewRGBA(image.Rect(0, 0, screenRegion["width"], screenRegion["height"]))
	for y := 0; y < screenRegion["height"]; y++ {
		for x := 0; x < screenRegion["width"]; x++ {
			idx := (y*screenRegion["width"] + x) * 3
			rgba.Set(x, y, color.RGBA{R: rgbData[idx], G: rgbData[idx+1], B: rgbData[idx+2], A: 255})
		}
	}

	// Обновляем изображение без пересоздания
	ff.image.Image = rgba
	ff.image.Refresh()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	ff := NewFocusFrame()
	ff.window.ShowAndRun()
}
