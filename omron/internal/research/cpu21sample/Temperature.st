FUNCTION_BLOCK Temperature
VAR_INPUT
    Raw : INT;
END_VAR
VAR_OUTPUT
    Result : REAL;
END_VAR
VAR
    Scaled : REAL;
    Converted : REAL;
END_VAR
Converted := INT_TO_REAL(Raw);
Scaled := 4.0 + (Converted / 4000.0 * 16.0);
Result := (Scaled - 4.0) / 16.0 * 90.0 - 20.0;
END_FUNCTION_BLOCK
