package transfers

import "sync"

type Manager struct {
	mu sync.RWMutex

	tasks map[string]*TransferTask
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*TransferTask),
	}
}

func (m *Manager) Add(task *TransferTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[task.ID] = task
}

func (m *Manager) List() []*TransferTask {

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []*TransferTask{}

	for _, t := range m.tasks {
		result = append(result, t)
	}

	return result
}
