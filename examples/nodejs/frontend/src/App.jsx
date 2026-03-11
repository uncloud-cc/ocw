import { useState, useEffect } from 'react';

const API_URL = '/api';

function App() {
  const [todos, setTodos] = useState([]);
  const [newTodo, setNewTodo] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetchTodos();
  }, []);

  async function fetchTodos() {
    try {
      const response = await fetch(`${API_URL}/todos`);
      if (!response.ok) throw new Error('Failed to fetch todos');
      const data = await response.json();
      setTodos(data);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  async function addTodo(e) {
    e.preventDefault();
    if (!newTodo.trim()) return;

    try {
      const response = await fetch(`${API_URL}/todos`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: newTodo }),
      });
      if (!response.ok) throw new Error('Failed to add todo');
      const todo = await response.json();
      setTodos([todo, ...todos]);
      setNewTodo('');
    } catch (err) {
      setError(err.message);
    }
  }

  async function toggleTodo(id) {
    try {
      const response = await fetch(`${API_URL}/todos/${id}`, {
        method: 'PATCH',
      });
      if (!response.ok) throw new Error('Failed to update todo');
      const updated = await response.json();
      setTodos(todos.map((t) => (t.id === id ? updated : t)));
    } catch (err) {
      setError(err.message);
    }
  }

  async function deleteTodo(id) {
    try {
      const response = await fetch(`${API_URL}/todos/${id}`, {
        method: 'DELETE',
      });
      if (!response.ok) throw new Error('Failed to delete todo');
      setTodos(todos.filter((t) => t.id !== id));
    } catch (err) {
      setError(err.message);
    }
  }

  if (loading) {
    return (
      <div className="container">
        <h1>OCW Todo App</h1>
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="container">
      <h1>OCW Todo App</h1>
      <p className="subtitle">
        Fullstack Node.js example with PostgreSQL
      </p>

      {error && <div className="error">{error}</div>}

      <form onSubmit={addTodo} className="add-form">
        <input
          type="text"
          value={newTodo}
          onChange={(e) => setNewTodo(e.target.value)}
          placeholder="What needs to be done?"
          className="input"
        />
        <button type="submit" className="btn btn-primary">
          Add
        </button>
      </form>

      <ul className="todo-list">
        {todos.length === 0 ? (
          <li className="empty">No todos yet. Add one above!</li>
        ) : (
          todos.map((todo) => (
            <li key={todo.id} className={`todo-item ${todo.completed ? 'completed' : ''}`}>
              <label className="todo-label">
                <input
                  type="checkbox"
                  checked={todo.completed}
                  onChange={() => toggleTodo(todo.id)}
                />
                <span className="todo-title">{todo.title}</span>
              </label>
              <button
                onClick={() => deleteTodo(todo.id)}
                className="btn btn-danger"
                aria-label="Delete todo"
              >
                Delete
              </button>
            </li>
          ))
        )}
      </ul>

      <footer className="footer">
        <p>
          Running with OCW workflow: PostgreSQL + Express API + Vite Dev Server
        </p>
      </footer>
    </div>
  );
}

export default App;
