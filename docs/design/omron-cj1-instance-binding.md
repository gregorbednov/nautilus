# CJ1: native instance binding и граница invocation — 2026-11-19

Обновление 20 ноября: причина девяти ошибок этого исследовательского сценария
выявлена и устранена штатным созданием исходного LD через Initialise/Compile.
[Новый отдельный корпус](omron-cj1-source-invocation.md) подтверждает полный
N1 длиной 3180 байт с вызовом FB. Приведённые ниже отрицательные результаты и старый
корпус сохранены неизменными; аппаратная семантика/загрузка не квалифицированы.

Штатный full writer теперь воспроизводимо создаёт объектные секции используемого
BLINK и экземпляра на CJ1M/CJ1H при current/check3.0 и4.0. Все четыре файла3092 bytes
побайтно восстановлены независимыми Go-кодеками. Это подтверждает структуру и
ссылки, но **не контракт параметрического вызова**: положительный контроль
использует прямой FBICALL без подготовки стека параметров.

Для полноценной invocation-оболочки совпали26 слов native-generated frame и
source compiler, однако последующий PLC_Program::Compile отвергает задачу с
девятью Insufficient input(s) diagnostics. Такой результат сохраняется как
отрицательный контроль; полная source invocation не объявляется готовой.

## Нативная последовательность

Опыт использует только собственные объекты в скрытом disposable x86 worker,
закреплённую CX-Pdata.dll и нормальные методы:

1. SetPLC, BCD, KBase current3/4; Program Check Version3/4 для каждого объекта.
2. SetUseFunctionBlocks, пять bank settings и stack AC0/64/1536.
3. Task type0/slot256, definition type32/number1024, instance type16/number0.
   SetFBBaseBodyNum(instance,1024); размеры семи registers2,0,0,3,0,0,0.
   Definition input/output counts4/2. Каждый owner привязан обычным
   MakeTaskConfigurationTable и проверен по getter-backed object relationship.
4. GenerateInst/AddToProgram создают15 инструкций определения и15 исходных
   инструкций кадра. FBIN и FBIRET добавляются native compiler.
5. Full writer получает task/definition/instance и selector65535. Каждый объект
   компилируется один раз; обе версии и фактический bool-аргумент отслеживаются
   [сохранённым guard observer](omron-cj1-program-guard-version.md).

Готовые compiled buffers, object flags и list pointers не подставляются.
GenerateInst проверяется через IsCompiled перед добавлением: return1 writer
и пустой общий error array сами по себе недостаточны. Ранний diagnostic опыт
это продемонстрировал: неверная запись нулевого indirect offset оставляла
нескомпилированные transfers, а writer мог создать укороченный файл.

Правильная запись stack indirect operands — `,-IR62`, `,-IR63`, `,IR62+`;
`0,-IR62` в этом пути не равнозначно ей. Финальный oracle останавливается
на такой ошибке до writer.

## Положительный контроль прямых ссылок

| Компонент | Результат |
| --- | --- |
| Task | LD W1.00 / FBICALL &0 &1024 / END;6 слов, errors0/warnings0 |
| Definition | BLINK, body1024;23 слова, errors0/warnings0 |
| Instance | instance0 → body1024;44 слова, errors0/warnings0 |
| Full writer | returned1, общий error array0, файл3092 bytes |

Native definition header хранит low10bits номера1024 как0; FBBN сохраняет1024.
Нативная instance section содержит один record54 words:10 header +44 code.
Директории: definitions330, instances380, tasks442; task record730.
Header word6 —08000000, библиотечный ExtendedTIMHold default. Это отдельный
нижнеуровневый рецепт; public full-image compiler сохраняет свой zero-options gate.

Go заново получает definition/instance из пользовательских CXP fixtures через
CompileCXPFBDefinition/CompileCXPFBInstance, кодирует LD/END и InstanceCall,
генерирует42 header words и собирает task configuration/directory/wrapper.
Сравниваются все773 words файла и код каждого из трёх program objects.
Четыре CPU/version сочетания дали одинаковые байты. Определение действительно
используемое: прежнего Unreferenced Function Block Body warning нет.

Отсутствие параметрического caller в этом контроле не доказывает исполнение
BLINK с нужными PV/PN/IN/OUT. ParameterStackContractQualified остаётся false.

## Invocation: точное совпадение до повторной проверки

Второй фиксированный рецепт использует GenerateInvocationCode(false),
AddOperandPreamble для UINT #100/#100 и BOOL4.14, выход BOOL W1.01 в postamble,
AddOperandPostamble для FBICALL &0 &1024 и GenerateEndOfInvocationCode.
Используются нормальные CObList API, без изменения объектов вручную.

Выходной scalar transfer вставляется в отдельный postamble; его добавление
сразу в preamble ставило бы его в начало. Нативные26 words окончательной
оболочки совпали с CompileCXPFBCallBody по реальному семиячеечному source rung
на обеих моделях и при обеих версиях. Enable LD проверяется отдельно.

Но full writer возвращает0: task имеет9 ошибок
`Insufficient input(s) for output instruction, check mnemonics`, definition и
instance —0 ошибок. Общий error array при этом0. Oracle проверяет также
индивидуальные program counts, помечает case accepted=false, не читает
Serialize после отказа и требует отсутствия выходного файла.

Причина отказа последующей проверки задачи пока не установлена. Ошибка теперь
локализована в lifecycle/контексте проверки generated invocation, а не в байтах
definition/instance или построенной оболочки. Нужны реальные caller ordering,
проверка/генерация до writer и контекст PLC_Program::Compile; подавление ошибок
и произвольная подстановка flags не допускаются как квалификация.

## Проверки и материалы

`InstanceBindingProbe` — дополнительный фиксированный research режим к
ProgramObjectProbe. Требует BCD и явных current/check3.0 либо4.0; принимает
только END-tagged recipes `direct-link` и `invocation-link`. Три последовательных
source transforms восстанавливаются offline; provenance содержит15 файлов.
Старые observer/current/guard вставки не менялись.

Portable corpus: `omron/compiler/testdata/native-instance-binding.json.gz`.
Go проверяет4 полных direct-файла,4 source invocation frames до отказа и
4 отказа full Compile, а также24 guard calls. Python replay даёт437 проверок
с raw captures; mutation tests запрещают потерю отказа и повышение semantic scope.
Все10 Omron packages, vet и51 Python tests прошли.

Raw final: `research/cj1-oracle/instance-binding-current{30,40}-20261119-06`.
Предыдущие01..05 — диагностические опыты, **не квалифицированные эталоны**;
один завершился ошибкой observer после отказа writer. Их логи сохранены отдельно.
Static Ghidra reports прослеживают ProgramCheck_FBInstance, DoAsicFixAndReturns,
full writer и CheckTaskProperties. ID проверяется в0..2047; writer связывает
program lists и форматирует секции через обычные compiler passes.

Общий пакет: `C:/cx-proger/research/hardware/20261119-cj1-instance-binding-01`.
Готовые директории имеют SHA256SUMS; исходники и переносимый corpus сохранены
в Git. Ранее frozen пакеты не изменялись. ПЛК, сеть и CX-One GUI не использовались.

Следующий этап — native lifecycle полноценного source invocation; затем
production explicit3.0 и отдельные protection/service/finalization/write/restore
gates. HardwareQualified, WriteQualified и SourceInvocationQualified — false.
