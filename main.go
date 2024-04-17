package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Generator генерирует последовательность чисел 1,2,3 и т.д. и
// отправляет их в канал ch. При этом после записи в канал для каждого числа
// вызывается функция fn. Она служит для подсчёта количества и суммы
// сгенерированных чисел.
func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	var i int64
	// 1. Функция Generator
	for {
		select {
		case <-ctx.Done():
			close(ch)
			return
		default:
			ch <- i
			fn(i)
			i++
		}
	}
}

// Worker читает число из канала in и пишет его в канал out.
func Worker(in <-chan int64, out chan<- int64) {
	ticker := time.NewTicker(time.Millisecond)
	// 2. Функция Worker
	for {
		<-ticker.C
		v, ok := <-in
		if !ok {
			close(out)
			break
		}
		out <- v
	}
}

func main() {
	chIn := make(chan int64)

	// 3. Создание контекста
	ctx := context.Background()
	ctxGenerator, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	// для проверки будем считать количество и сумму отправленных чисел
	var inputSum atomic.Int64   // сумма сгенерированных чисел
	var inputCount atomic.Int64 // количество сгенерированных чисел

	// генерируем числа, считая параллельно их количество и сумму
	go Generator(ctxGenerator, chIn, func(i int64) {
		inputSum.Add(i)
		inputCount.Add(1)
	})

	var wg sync.WaitGroup

	const NumOut = 15 // количество обрабатывающих горутин и каналов
	// outs — слайс каналов, куда будут записываться числа из chIn
	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		// создаём каналы и для каждого из них вызываем горутину Worker
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i])
	}

	// amounts — слайс, в который собирается статистика по горутинам
	amounts := make([]int64, NumOut)
	// chOut — канал, в который будут отправляться числа из горутин `outs[i]`
	chOut := make(chan int64, NumOut)

	// 4. Собираем числа из каналов outs
	for i, v := range outs {
		wg.Add(1)
		go func(in <-chan int64, i int64) {
			for num := range in {
				chOut <- num
				amounts[i]++
			}
			wg.Done()
		}(v, int64(i))
	}

	go func() {
		// ждём завершения работы всех горутин для outs
		wg.Wait()
		// закрываем результирующий канал
		close(chOut)
		cancel()
	}()

	var count int64 // количество чисел результирующего канала
	var sum int64   // сумма чисел результирующего канала

	// 5. Читаем числа из результирующего канала
	for v := range chOut {
		sum += v
	}
	for _, v := range amounts {
		count += v
	}

	fmt.Println("Количество чисел", inputCount.Load(), count)
	fmt.Println("Сумма чисел", inputSum.Load(), sum)
	fmt.Println("Разбивка по каналам", amounts)

	// проверка результатов
	if inputSum.Load() != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum.Load(), sum)
	}
	if inputCount.Load() != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount.Load(), count)
	}
	for _, v := range amounts {
		inputCount.Add(-v)
	}
	if inputCount.Load() != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}