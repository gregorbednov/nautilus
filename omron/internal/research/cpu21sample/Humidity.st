FUNCTION_BLOCK Humidity
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
Scaled := Converted / 250.0 + 4.0;
Result := (Scaled - 4.0) / 16.0 * 95.0;
END_FUNCTION_BLOCK
