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

	select {
	case <-ctx.Done(): // Проверка на сигнал остановки
		log.Println("Получен сигнал остановки. Прерывание проверки.")
		resp.Body.Close()
		return
	default:
		result := CheckResult{Success: success}
		results <- result
	}
}

func runTests(ctx context.Context, interval time.Duration, numChecks int) TestResult {
	// Создаем каналы для результатов и сигналов для остановки
	results := make(chan CheckResult, numChecks)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	wg := sync.WaitGroup{}

	for i := 0; i < numChecks; i++ {
		wg.Add(1)
		go performCheck(ctx, &wg, results)

		select {
		case <-ctx.Done(): // Проверка на сигнал остановки
			log.Println("Получен сигнал остановки. Прерывание тестов.")
			wg.Wait()
			close(results)
			return collectResults(results)
		default:
			time.Sleep(interval)
		}
	}

	wg.Wait()
	close(results)

	return collectResults(results)
}

func collectResults(results <-chan CheckResult) TestResult {
	testResult := TestResult{
		Results: make([]CheckResult, 0),
	}

	for result := range results {
		testResult.Results = append(testResult.Results, result)
	}

	return testResult
}

func main() {
	interval := flag.Duration("t", 3*time.Second, "Интервал между запусками проверок")
	numChecks := flag.Int("n", 3, "Количество проверок")
	flag.Parse()

	log.Println("Запуск утилиты для измерения производительности и оценки отказоустойчивости API...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop
		log.Println("Получен сигнал остановки. Сохранение результатов в файл...")

		cancel() // Отменяем контекст после получения сигнала
	}()

	testResult := runTests(ctx, *interval, *numChecks)

	// Выводим и анализируем результаты
	successfulCount := 0
	for _, result := range testResult.Results {
		if result.Success {
			successfulCount++
		}
	}

	successfulPercentage := float64(successfulCount) / float64(len(testResult.Results)) * 100
	fmt.Printf("Процент успешных запросов: %.2f%%\n", successfulPercentage)

	// Сохраняем результаты в файл
	jsonData, err := json.MarshalIndent(testResult, "", "    ")
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
	log.Println("Работа программы завершена.")
}
