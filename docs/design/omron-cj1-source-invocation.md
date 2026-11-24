# CJ1: полный N1 с вызовом FB из исходного проекта

20 ноября 2026. Закрыта причина отказа исследовательского сценария из
[предыдущего этапа](omron-cj1-instance-binding.md). Независимые публичный
`compiler.CompileCXPImage` и CLI `omron compile-image` воспроизводят полный
нативный N1 для проекта с одним вызовом BLINK, определением и экземпляром.
DLL используется только для получения эталонов; сборка и повторная проверка
работают без CX-One и Windows.

| Профиль | KBase current | Program Check Version | Результат |
| --- | --- | --- | --- |
| CJ1M CPU21 | 3.0 | 3.0 | 3180 байт, ошибок/предупреждений 0 |
| CJ1H-H CPU65 | 3.0 | 3.0 | те же 3180 байт |
| CJ1M CPU21 | 4.0 | 4.0 | те же 3180 байт |
| CJ1H-H CPU65 | 4.0 | 4.0 | те же 3180 байт |

SHA-256 всех четырёх файлов:
`0345bfbcfcedbc88cd90759cf30e7948ecad94d805cb1e5e040acd101dece502`.
Сравнивается весь файл: заголовок, таблицы, ссылки, определение, экземпляр,
задача и завершители. Прежнее совпадение одного тела вызова этого не доказывало.

## Причина девяти ошибок

Pinned CX-Pdata SHA-256:
`a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141`.
`ProgramCheckStack` по адресу `121f2b67` работает в фазе 3 при бите `0x800`
Program Check Flags и пропускает инструкцию/определение с `0x10000000`.
Craft Anything evidence:
`ev_063b1ae7d56afbf00089da3e4a5f9be775f48bc985c9ebe557812ea389bbebc9`,
finding `fnd_9`. Ghidra-декомпиляция сохранена отдельно. Это статические
сведения; runtime-журналы собственного worker являются внешними артефактами.

Старый сценарий создавал исходный `LD W1.00` через `GenerateInst`, получая
флаги `0x100100c0`. Перед проверкой FBCALL счётчики были `[0,16,0,0]`,
FBCALL и восемь вложенных инструкций получали Insufficient input.
Новый сценарий использует штатные конструктор, `InstructionInst::Initialise`,
`Compile`, затем добавление исходного LD в программу: флаги `0xc0`, то же
машинное слово. Перед FBCALL штатный поток обработки даёт `[1,16,0,1]`.
Это не означает, что счётчики меняет именно ProgramCheckStack при проверке LD:
изменение происходит между наблюдаемыми вызовами.

Флаги, счётчики, compiled buffers и возвращаемые значения не подменялись.
Read-only hook записал 208 проверок: checkFlags `0x800`, special byte 0.
Отрицательный опыт `invocation-no-enable` убирает LD: оба CPU при обеих версиях
получают 9 ошибок, writer=0, файла нет. Проверка входов осталась активной.

Диагностические попытки сохранены отдельно: корень через фабрику дал 8 ошибок;
генерация всех детей остановилась на IsCompiled; первый исходный LD дал
writer=1, но старый наблюдатель не прошёл собственную проверку GetBuffer.
Они не считаются положительным корпусом. Новый наблюдатель берёт код задачи
из записанного N1, проверяет границы и нативную длину, не вызывая повторно
GetBuffer вложенной задачи. Определение/экземпляр читаются прежним способом.

## Переносимое воспроизведение

`omron/compiler/testdata/n1-source-invocation.cxt`: задача Probe, один
графический rung с LD и BLINK_L1, затем END. Параметры UINT `#100`, UINT `#100`,
BOOL `4.14`, выход BOOL `W1.01`. Определение BLINK взято из предоставленного
CJ1M-проекта; оставлен один экземпляр. Код задачи 28 слов, определения 23,
экземпляра 44. Четыре опции заголовка выключены, память явно задана в
`n1-example-settings.json`; её соответствие установке этим опытом не доказано.

Из корня репозитория, с уже собранным `naut`:

```text
naut omron compile-image -model "CJ1M CPU21" -resource "Resource[0]" -timer-mode bcd -settings omron/compiler/testdata/n1-example-settings.json -o source-invocation.n1 omron/compiler/testdata/n1-source-invocation.cxt
```

Второй профиль — `CJ1H-H CPU65`; output должен быть новым.
`VerifyCXPImageBuild` повторно разбирает точный исходник и сверяет сборку.
Корпус `native-source-invocation.json.gz` содержит два журнала: 12 case,
36 Compile с явным Program Check Version, 208 stack events и provenance.
Из восьми положительных файлов четыре — полные вызовы, четыре — прямые ссылки.

```text
python -B tools/omron-research/verify_cj1_source_invocation.py
python -B tools/omron-research/verify_cj1_source_invocation.py --capture-root C:/cx-proger/research/cj1-oracle
python -B -m unittest discover -s tools/omron-research -p "test_*.py"
go test ./omron/...
go test ./cmd/naut -run Omron
go vet ./omron/...
```

С capture-root проверяются raw events, все 17 provenance entries, точное
восстановление четырёх вставок наблюдателя и N1. Нативный повтор требует
Windows, pinned DLL/Frida, нового output и параметров:
`-ProgramObjectProbe -InstanceBindingProbe -InvocationStackTraceProbe
-BindingInputLifecycle source -TimerCounterMode bcd -LocalCurrentVersion 3.0
-ProgramCheckVersion 3.0 -Cases CASES -Output NEW` для
`Invoke-CJ1InstructionOracle.ps1`. CASES — `runs[].request` из корпуса.

Историческое общее поле provenance `fullProgramCompiled` осталось false;
оно не используется как доказательство результата. Успех устанавливается
по трассам Compile, per-program errors/warnings, ответу writer и точным файлам.

## Границы и следующий этап

Доказана файловая эквивалентность одного скалярного FB-рецепта в BCD.
Выполнение FB на ПЛК, аппаратный parameter stack contract, storage initialization,
произвольные FB/типы параметров, вся LD-палитра и binary FB ещё не проверены.
`hardwareQualified`, `writeQualified`, `parameterStackContractQualified` — false.
Сетевых обращений и записей UM в этом этапе не было.

Непустой `codec.Profile.UnitVersion`, включая `3.0`, по-прежнему отклоняется.
Далее — проверка всех реализованных форм под **Program Check Version 3.0**,
включая запрещённые формы. Одиночные instruction-пробы и этот FB-рецепт не дают
общего разрешения версии. Затем — защита, 0307/0317, финализация,
контролируемая запись и доказанное восстановление исходной программы.

Локальный архив:
`C:/cx-proger/research/hardware/20261119-cj1-invocation-lifecycle-01`.
Raw эталоны в `research/cj1-oracle`:
`invocation-source-lifecycle-current30-20261119-02` и
`invocation-source-lifecycle-current40-20261119-02`.
Семь новых capture-каталогов, включая диагностические, перечислены в
`captures.json`; `SHA256SUMS` фиксирует raw-данные, исходники и проверки.
Предыдущие архивы и отрицательный instance-binding корпус неизменны.
