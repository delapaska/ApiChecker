package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type CheckResult struct {
	Success bool `json:"success"`
	// дополнительные поля, если нужно
}

type TestResult struct {
	Results []CheckResult `json:"results"`
	// дополнительные поля, если нужно
}

func performCheck(ctx context.Context, wg *sync.WaitGroup, results chan<- CheckResult) {
	defer wg.Done()

	// Здесь выполняется одна проверка
	// Пример:
	resp, err := http.Get("https://thecatapi.com")
	if err != nil {
		log.Println("Ошибка при выполнении запроса:", err)
		results <- CheckResult{Success: false}
		return
	}

	// Проверяем успешность запроса и выполняем дополнительные проверки, если нужно
	success := resp.StatusCode == http.StatusOK

	results <- CheckResult{Success: success}
}

func runTests(ctx context.Context, interval time.Duration, numChecks int) {
	// Создаем каналы для результатов и сигналов для остановки
	results := make(chan CheckResult, numChecks)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	wg := sync.WaitGroup{}

	for i := 0; i < numChecks; i++ {
		wg.Add(1)
		go performCheck(ctx, &wg, results)

		time.Sleep(interval)
		select {
		case <-stop: // Проверка на сигнал остановки
			log.Println("Получен сигнал остановки. Завершение проверок.")
			close(results)
			wg.Wait()
			return
		default:
		}
	}

	wg.Wait()
	close(results)

	// Собираем результаты
	testResult := TestResult{
		Results: make([]CheckResult, 0, numChecks),
	}

	for result := range results {
		testResult.Results = append(testResult.Results, result)
	}

	// Выводим и анализируем результаты
	successfulCount := 0
	for _, result := range testResult.Results {
		if result.Success {
			successfulCount++
		}
	}

	successfulPercentage := float64(successfulCount) / float64(numChecks) * 100
	fmt.Printf("Процент успешных запросов: %.2f%%\n", successfulPercentage)

	// Сохраняем результаты в файл
	jsonData, err := json.Marshal(testResult)
	if err != nil {
		log.Println("Ошибка при сериализации результатов в JSON:", err)
		return
	}

	err = ioutil.WriteFile("test_results.json", jsonData, 0644)
	if err != nil {
		log.Println("Ошибка при сохранении результатов в файл:", err)
		return
	}

	log.Println("Результаты успешно сохранены в файл test_results.json.")
}

func main() {
	interval := flag.Duration("t", 10*time.Second, "Интервал между запусками проверок")
	numChecks := flag.Int("n", 10, "Количество проверок")
	flag.Parse()

	log.Println("Запуск утилиты для измерения производительности и оценки отказоустойчивости API...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	runTests(ctx, *interval, *numChecks)
}
