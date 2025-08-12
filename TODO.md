 - [ ] сделать возможность перевода на выбранный язык. то есть метка с фронта приходит с языком, и добавляем в промпт обработки ллм просьбу перевести текст на <language>
 - [ ] есть проблема с тем, что при stop не успевает обрабатываться последний чанк записи, мы получаем ответ сразу не дожидаясь


### ошибка

тут вот получаем ошибку какую то в духе что слишком малый буфер. может это и не проблема?
а еще может быть все довольно долго
видимо, нужно ставить больше таймаут и делать на стороне приложения лоадер/блокировать новые запросы и возможность вручную отменить


2025-07-24T08:54:07+03:00 ERR Session f728ce42-a0b2-4596-9026-2150512f6030: Received error from Realtime API: &{Code:input_audio_buffer_commit_empty Message:Error committing input audio buffer: buffer too small. Expected at least 100ms of audio, but buffer only has 0.00ms of audio. Param:}
2025-07-24T08:54:37+03:00 WRN Session f728ce42-a0b2-4596-9026-2150512f6030: Timeout waiting for next completion event after 30s
2025-07-24T08:54:37+03:00 INF Realtime result channel closed for session: f728ce42-a0b2-4596-9026-2150512f6030
2025-07-24T08:54:37+03:00 INF Cancelled realtime transcription session: f728ce42-a0b2-4596-9026-2150512f6030
2025-07-24T08:54:37+03:00 INF Session f728ce42-a0b2-4596-9026-2150512f6030: Connection closed normally
2025-07-24T08:54:37+03:00 DBG Generating response with OpenAI Responses API input="Please correct all spelling, grammar, and punctuation in the following transcribed text. Add spaces, split sentences, and create new paragraphs where appropriate for better readability. Do not add, remove, or change any information. Do not paraphrase or interpret. Only return the corrected, well-formatted text. Text: `Еще одна проверка записи. Я теперь подожду чуть-чуть подольше, чтобы убедиться, что я примерно правильно предполагаю проблему, с которой я столкнулся.Именно.`" model=gpt-4.1-nano
2025-07-24T08:54:37+03:00 INF Session f728ce42-a0b2-4596-9026-2150512f6030: Exiting OpenAI Realtime API message handler
2025-07-24T08:54:39+03:00 DBG Successfully generated response with OpenAI model=gpt-4.1-nano-2025-04-14 outputPreview="Еще одна проверка записи. Я ...
