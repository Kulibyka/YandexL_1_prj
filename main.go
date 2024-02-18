package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Operation struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
}

type Task struct {
	ID         int        `json:"id"`
	Expression string     `json:"expression"`
	Status     string     `json:"status"`
	Result     float64    `json:"result,omitempty"`
	StartTime  time.Time  `json:"start_time,omitempty"`
	EndTime    time.Time  `json:"end_time,omitempty"`
	AgentID    int        `json:"agent_id,omitempty"`
	Mutex      sync.Mutex `json:"-"`
}

var (
	tasks      []*Task
	operations = []Operation{
		{"Сложение", 20 * time.Second},
		{"Вычитание", 30 * time.Second},
		{"Умножение", 25 * time.Second},
		{"Деление", 50 * time.Second},
	}
	nextTaskID  = 1
	taskChannel = make(chan *Task, 100)
)

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	// Распаковываем JSON-данные из запроса
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, "Failed to decode JSON", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	// Добавляем задачу в список
	task.ID = nextTaskID
	nextTaskID++
	task.Status = "queued"
	task.Mutex = sync.Mutex{} // Инициализируем Mutex для безопасного доступа к задаче
	tasks = append(tasks, &task)

	// Отправляем задачу в канал для агента
	taskChannel <- &task

	// Отвечаем клиенту с ID задачи
	json.NewEncoder(w).Encode(map[string]int{"task_id": task.ID})
}

func getTaskResultHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID задачи из URL
	segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/tasks/"), "/")
	taskID, err := strconv.Atoi(segments[0])
	if err != nil {
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	// Проверяем, существует ли задача с указанным ID
	if taskID < 1 || taskID > len(tasks) {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Получаем задачу по ее ID
	task := tasks[taskID-1]

	// Блокируем Mutex для безопасного доступа к задаче
	task.Mutex.Lock()
	defer task.Mutex.Unlock()

	// Если задача еще не завершена, отправляем сообщение ожидания
	if task.Status != "completed" {
		w.WriteHeader(http.StatusAccepted) // Статус код 202 Accepted
		json.NewEncoder(w).Encode(map[string]string{"status": "Task is not completed yet"})
		return
	}

	// Отвечаем клиенту с результатом задачи
	response := map[string]float64{"result": task.Result}
	json.NewEncoder(w).Encode(response)
}

func listTasksHandler(w http.ResponseWriter, r *http.Request) {
	// Отвечаем клиенту со списком задач
	json.NewEncoder(w).Encode(tasks)
}

func getOperationsHandler(w http.ResponseWriter, r *http.Request) {
	// Отправляем клиенту список доступных операций со временем их выполнения
	json.NewEncoder(w).Encode(operations)
}

func getTaskForExecutionHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем задачу для выполнения от оркестратора
	task := <-taskChannel

	// Отправляем задачу клиенту
	json.NewEncoder(w).Encode(task)
}

func receiveTaskResultHandler(w http.ResponseWriter, r *http.Request) {
	// Распаковываем JSON-данные из запроса
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Находим соответствующую задачу в списке
	for _, t := range tasks {
		if t.ID == task.ID {
			// Обновляем статус и результат задачи
			t.Status = "completed"
			t.Result = task.Result
			t.EndTime = time.Now()
			break
		}
	}
}

// Другие обработчики для остальных эндпоинтов...

// Агент (вычислитель)
func startAgents(numAgents int) {
	for i := 0; i < numAgents; i++ {
		go func(agentID int) {
			for task := range taskChannel {
				// Получаем задачу из канала
				task.Status = "calculated"
				task.StartTime = time.Now()

				// Выполняем вычисление
				task.Mutex.Lock()
				task.Result = evaluateExpression(task.Expression)
				task.Mutex.Unlock()

				task.Status = "completed"
				task.EndTime = time.Now()
			}
		}(i + 1)
	}
}

func evaluateExpression(expression string) float64 {
	// Удаляем все пробелы из выражения
	expression = strings.ReplaceAll(expression, " ", "")

	// Создаем стеки для операндов и операторов
	operandStack := make([]float64, 0)
	operatorStack := make([]rune, 0)

	// Функция для выполнения операции
	performOperation := func() {
		if len(operandStack) < 2 || len(operatorStack) == 0 {
			return
		}

		b := operandStack[len(operandStack)-1]
		operandStack = operandStack[:len(operandStack)-1]

		a := operandStack[len(operandStack)-1]
		operandStack = operandStack[:len(operandStack)-1]

		op := operatorStack[len(operatorStack)-1]
		operatorStack = operatorStack[:len(operatorStack)-1]

		var result float64
		switch op {
		case '+':
			time.Sleep(operations[0].Duration)
			result = a + b
		case '-':
			time.Sleep(operations[1].Duration)
			result = a - b
		case '*':
			time.Sleep(operations[2].Duration)
			result = a * b
		case '/':
			time.Sleep(operations[3].Duration)
			result = a / b
		}
		operandStack = append(operandStack, result)
	}

	// Обходим каждый символ в выражении
	for _, char := range expression {
		switch char {
		case '(':
			operatorStack = append(operatorStack, char)
		case ')':
			for len(operatorStack) > 0 && operatorStack[len(operatorStack)-1] != '(' {
				performOperation()
			}
			if len(operatorStack) > 0 && operatorStack[len(operatorStack)-1] == '(' {
				operatorStack = operatorStack[:len(operatorStack)-1]
			}
		case '+', '-':
			for len(operatorStack) > 0 && (operatorStack[len(operatorStack)-1] == '+' || operatorStack[len(operatorStack)-1] == '-' || operatorStack[len(operatorStack)-1] == '*' || operatorStack[len(operatorStack)-1] == '/') {
				performOperation()
			}
			operatorStack = append(operatorStack, char)
		case '*', '/':
			for len(operatorStack) > 0 && (operatorStack[len(operatorStack)-1] == '*' || operatorStack[len(operatorStack)-1] == '/') {
				performOperation()
			}
			operatorStack = append(operatorStack, char)
		default:
			// Если символ - цифра или точка, добавляем ее в стек операндов
			operand, _ := strconv.ParseFloat(string(char), 64)
			operandStack = append(operandStack, operand)
		}
	}

	// Выполняем все оставшиеся операции
	for len(operatorStack) > 0 {
		performOperation()
	}

	// Возвращаем результат
	return operandStack[0]
}

func main() {

	go startAgents(3)

	router := mux.NewRouter()
	router.HandleFunc("/tasks/add", addTaskHandler).Methods("POST")
	router.HandleFunc("/tasks", listTasksHandler).Methods("GET")
	router.HandleFunc("/tasks/{id}/result", getTaskResultHandler).Methods("GET")
	router.HandleFunc("/operations", getOperationsHandler).Methods("GET")
	router.HandleFunc("/tasks/execute", getTaskForExecutionHandler).Methods("GET")
	router.HandleFunc("/tasks/result", receiveTaskResultHandler).Methods("POST")

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
