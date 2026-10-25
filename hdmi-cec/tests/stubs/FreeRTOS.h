#pragma once
#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>
typedef uint32_t TickType_t;
typedef unsigned UBaseType_t;
typedef void *TaskHandle_t;
#define pdTRUE 1
#define pdMS_TO_TICKS(x) ((TickType_t)(x))
#define eSetBits 1
#define eNoAction 0
#define portMAX_DELAY UINT32_MAX
