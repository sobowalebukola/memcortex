
ollama serve &


echo "Waiting for Ollama server to start..."
while ! ollama list > /dev/null 2>&1; do
  sleep 2
done


echo "Checking models..."
ollama pull nomic-embed-text
ollama pull deepseek-r1:1.5b


wait