package main

import (
        "bufio"
        "fmt"
        "html/template"
        "log"
        "net/http"
        "os"
        "strconv"
        "strings"
        "time"
)

// Структура для хранения информации об одной аренде DHCP
type Lease struct {
        ExpiryTime time.Time // Время истечения аренды (конвертрованное)
        MACAddress string    // MAC-адрес клиента
        IPAddress  string    // Выданный IP-адрес
        Hostname   string    // Имя хоста клиента (может быть '*' или отсутствовать)
        ClientID   string    // Идентификатор клиента (опционально, часто совпадает с MAC)
}

// Путь к файлу dnsmasq.leases
// На некоторых системах может быть другим, например /var/db/dnsmasq.leases
const leasesFilePath = "/var/lib/misc/dnsmasq.leases"

// Функция для парсинга файла dnsmasq.leases
func parseLeases(filePath string) ([]Lease, error) {
        file, err := os.Open(filePath)
        if err != nil {
                // Добавляем контекст к ошибке
                return nil, fmt.Errorf("не удалось открыть файл %s: %w", filePath, err)
        }
        defer file.Close() // Гарантируем закрытие файла

        var leases []Lease
        scanner := bufio.NewScanner(file)

        // Читаем файл построчно
        for scanner.Scan() {
                line := scanner.Text()
                fields := strings.Fields(line) // Разделяем строку по пробелам

                // Стандартный формат: <unix_timestamp> <mac_address> <ip_address> <hostname> <client_id>
                // Иногда client_id может отсутствовать
                if len(fields) < 4 {
                        log.Printf("Предупреждение: пропущена строка с недостаточным количеством полей: %s", line)
                        continue // Пропускаем некорректные строки
                }

                // 1. Парсим время истечения (Unix timestamp)
                expiryUnix, err := strconv.ParseInt(fields[0], 10, 64)
                if err != nil {
                        log.Printf("Предупреждение: не удалось распарсить временную метку в строке '%s': %v", line, err)
                        continue
                }
                expiryTime := time.Unix(expiryUnix, 0) // Конвертируем в time.Time

                // 2. Извлекаем MAC-адрес
                macAddress := fields[1]

                // 3. Извлекаем IP-адрес
                ipAddress := fields[2]

                // 4. Извлекаем имя хоста
                hostname := fields[3]
                if hostname == "*" {
                        hostname = "N/A" // Используем "N/A" для неизвестных хостов
                }

                // 5. Извлекаем Client ID (опционально)
                clientID := "N/A" // Значение по умолчанию
                if len(fields) > 4 {
                        clientID = fields[4]
                }

                // Добавляем распарсенную аренду в срез
                leases = append(leases, Lease{
                        ExpiryTime: expiryTime,
                        MACAddress: macAddress,
                        IPAddress:  ipAddress,
                        Hostname:   hostname,
                        ClientID:   clientID,
                })
        }

        // Проверяем на ошибки во время сканирования файла
        if err := scanner.Err(); err != nil {
                return nil, fmt.Errorf("ошибка при чтении файла %s: %w", filePath, err)
        }

        return leases, nil
}

// Обработчик HTTP-запросов
func leaseHandler(w http.ResponseWriter, r *http.Request) {
        leases, err := parseLeases(leasesFilePath)
        if err != nil {
                log.Printf("Ошибка парсинга файла leases: %v", err)
                http.Error(w, fmt.Sprintf("Не удалось прочитать или обработать файл leases: %v", err), http.StatusInternalServerError)
                return
        }

        // Определяем HTML шаблон для таблицы
        // Используем html/template для безопасной генерации HTML
        // --- ИСПРАВЛЕНИЯ В ШАБЛОНЕ ---
        tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Dnsmasq Leases</title>
    <style>
        body { font-family: sans-serif; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        tr:nth-child(even) { background-color: #f9f9f9; }
    </style>
</head>
<body>
    <h1>Активные аренды DHCP (dnsmasq)</h1>
    <table>
        <thead>
            <tr>
                <th>Время истечения</th>
                <th>MAC Адрес</th>
                <th>IP Адрес</th>
                <th>Имя хоста</th>
                <th>Client ID</th>
            </tr>
        </thead>
        <tbody>
            {{/* ИЗМЕНЕНИЕ: Итерируем по полю .Leases структуры данных */}}
            {{range .Leases}}
            <tr>
                {{/* Внутри range, точка (.) теперь ссылается на текущий элемент Lease */}}
                <td>{{.ExpiryTime.Format "2006-01-02 15:04:05 MST"}}</td>
                <td>{{.MACAddress}}</td>
                <td>{{.IPAddress}}</td>
                <td>{{.Hostname}}</td>
                <td>{{.ClientID}}</td>
            </tr>
            {{else}}
            <tr>
                <td colspan="5">Нет активных аренд или не удалось прочитать файл.</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{/* ИЗМЕНЕНИЕ: Доступ к полям верхнего уровня структуры данных */}}
    <p>Данные из: {{.LeasesFilePath}}</p>
        <p>Обновлено: {{.CurrentTime.Format "2006-01-02 15:04:05 MST"}}</p>
</body>
</html>
`

        t, err := template.New("leases").Parse(tmpl)
        if err != nil {
                log.Printf("Ошибка парсинга HTML шаблона: %v", err)
                http.Error(w, "Внутренняя ошибка сервера (шаблон)", http.StatusInternalServerError)
                return
        }

        // Готовим данные для передачи в шаблон (структура та же)
        data := struct {
                Leases         []Lease
                LeasesFilePath string
                CurrentTime    time.Time
        }{
                Leases:         leases,
                LeasesFilePath: leasesFilePath,
                CurrentTime:    time.Now(),
        }

        w.Header().Set("Content-Type", "text/html; charset=utf-8")

        err = t.Execute(w, data)
        if err != nil {
                // Логируем ошибку выполнения, которая теперь не должна возникать из-за доступа к данным
                log.Printf("Ошибка выполнения HTML шаблона: %v", err)
                // Не отправляем http.Error здесь, так как заголовок мог уже быть отправлен
        }
}

// Остальной код (main, parseLeases, Lease struct) остается без изменений.
// Не забудьте импортировать все нужные пакеты.
func main() {
        // Регистрируем обработчик для корневого пути "/"
        http.HandleFunc("/", leaseHandler)

        // Определяем порт для прослушивания
        port := "8090"
        addr := ":" + port

        log.Printf("Запуск веб-сервера по адресу http://localhost:%s", port)
        log.Printf("Сервер будет читать данные из %s", leasesFilePath)
        log.Println("Для остановки сервера нажмите Ctrl+C.")

        // Запускаем HTTP сервер
        // ListenAndServe блокирует выполнение до ошибки или остановки сервера
        err := http.ListenAndServe(addr, nil)
        if err != nil {
                log.Fatalf("Не удалось запустить сервер: %v", err)
        }
}
