# Наблюдённый compiler selector и привязка файла

18 ноября 2026 года. Продолжение [per-task writer](omron-task-writer-and-record-extraction.md).
Усилен переносимый аудитор наблюдений: сохраняются selector задачи, raw mode
и оба исходных пути; файл сопоставляется с выбранным путём из compile-enter.
Новый standalone task golden пока не получен.

## Изменения аудитора

Наблюдательcapture-cj1.js уже записывал эти поля. В Go event/compilation они
раньше терялись. Теперь `audit-capture` выдаёт по каждой операции:

- `taskSelector`: uint32, включая0; отсутствие остаётсяnil/неизвестным;
- `mode`: raw int32 compiler argument; **не режим исполнения PLC**;
- `selectionKind`: `whole-image-selector` дляffff, `explicit-selector` для другого
  наблюдённого значения, `unobserved` для старых записей без поля;
- `explicitOutputPath`, `defaultOutputPath`, `expectedFilePath`, `observedFilePath`;
- `filePathVerified`: точное совпадение строк captured path и ожидаемого пути.

Приffff writer использует переданный ему список программ. Это не доказывает,
что список содержал весь проект или всё содержимое PLC. Проверяется значение
селектора полного образа; полнота проекта и source identity не повышаются.

Task selector отличается от `profile.selector`: последний относится к CPU
profile и не является номером задачи. Числовой explicit-selector сам по себе
не доказывает допустимость задачи или формат полученного файла.

Если explicit path непустой, ожидается он; иначе берётся default path.
Если известный ожидаемый путь не совпал с compiler-file.path, capture считается
неполным. Нормализация Windows/Linux paths и проверка реальной файловой
тождественности не выполняются: сравниваются строки исходного transcript.
Это сохраняет переносимость анализа старых Windows observations на Linux.

Старые записи без task/mode/path остаются допустимыми в рамках прежней
проверки lifecycle, но недостающие факты не выдумываются. Нулевые task/mode
сохраняются в JSON. Отрицательный/слишком большой task и mode за пределами
int32, а также дробные значения отвергаются как неверные события.

`verify` для текущего **full N1** корпуса теперь отвергает наблюдённый task
selector, отличный отffff. Результат содержит `taskSelector` и отдельный
`wholeImageSelectorVerified`. Если старый capture не содержал selector,
второй признакfalse; совпадение bytes всё ещё учитывается отдельно.
Ни source identity, ни hardware qualification этим не повышаются.

## Что реально содержат старые captures

Повторно собраннымbin/cj1-oracle.exe проверены три исходных transcript:

| Case | Task selector / mode | Complete | File path verified |
| --- | --- | --- | --- |
|CJ1M baseline |ffff /1 |true |true |
|CJ1M input-bit |ffff /1 |true |true |
|CJ1H baseline |ffff /1 |false |false |

CJ1H остаётся историческим operator-copy без captured file: audit CLI
заканчивается ожидаемым exit1. Verifier полного корпуса проходит, сохраняя
его прежние ограничения. Новых DLL calls или native compile captures не было.
Следовательно, эти три образца нельзя переименовать в standalone task goldens.

## F7 и Ctrl+F7: что установлено статически

В установленномcx-pres.dll, SHA-256
`3162468b3a3adc03d717aaa758b1e938e561979e8bbe2cd48ffb42bc636134db`,
найденыRT_MENU/RT_ACCELERATOR entries:

| Команда | ID | Accelerator |
| --- | --- | --- |
|PLC → Compile All PLC Programs |39010 /9862hex |F7 |
|Program → Compile |39188 /9914hex |Ctrl+F7 |

В CX-P.exe command-map entry1290ad40 связывает39010 с124c9dd0;
entry12909558 связывает39188 с12496510. В основной ветви12496510 вызывает
12496570. Последний передаёт12 явных аргументов common worker124b7fa0;
первые3 — адреса контейнеров, остальные9 — нули. Callee завершаетRET30.

При base stack delta12+1ac+16 =1c8, inputs поESP+1e4/1f0/1f4 соответствуют
аргументам7/10/11. Все три равны0. Если успешный путь достигает gate записи,
он проходит124b8b81→124b8d76→124b8fed→124b90ac и передаёт в writer selectorffff
по124b90ba. Поэтому **название Program Compile не доказывает standalone task
output**: в этой разобранной ветви выбирается whole-program writer.

Другие editor/document ветви поtype6/8/10 и дополнительные command-map handlers
не объявлены исследованными. Ghidra вывела слишком широкие диапазоны для
127aa720/127bfd00; эти listings исключены из переносимого доказательства.
Переносимые4 functions —12496510,124c9dd0,12496570,124b7fa0.

## Проверки и следующий шаг

Добавлены проверки task0/mode0, unknown legacy metadata, explicit/default path,
несовпавшего/пропавшего пути, Unicode paths, int bounds и подмены full N1
corpus capture на task selector. Мутации выполняются в test temp copies;
реальные fixture bytes не изменены и не объявляются новыми наблюдениями.

Прошли `go test ./tools/omron-research/oracle`,
`go vet ./tools/omron-research/oracle`, build и CLI corpus verification.
Offline verifier: **380/380**, включая native E8 calls, menu/accelerator bytes,
command-map entries и неизменность квалификации при replay старых captures.

В Git: `tools/omron-research/oracle/testdata/evidence/compile-menu-evidence.json.gz`.
SHA-256: `e876f58728a2451afc4db85d43ef35f33490a0796c2a50f785cc38ed965ea8c5`.
Raw: `research/hardware/20261118-cj1-capture-selection-01`.
Manifest: 31 файлов, SHA-256 `09ac803e9d2662fa831695d3f983b5399ac78342c0160e2030af0548b1184828`.

В сессии отсутствует штатныйnode_repl runtime управления GUI, CX-P не был
запущен. Перед новой попыткой capture нужно исследовать локальный export /
selected-task path с ненулевым selector, а не полагаться на Ctrl+F7. Затем
сравнить настоящий файл с ExtractN1TaskRecord и проверить auxiliary/FB blocks.
Transfer action на действующей установке для этого не используется.
Сети и vendor execution не было; .37 READ ONLY, .36 offline.
