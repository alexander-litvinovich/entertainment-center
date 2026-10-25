#pragma once
#include "FreeRTOS.h"
void vTaskDelay(TickType_t);
uint32_t ulTaskNotifyTakeIndexed(unsigned, int, TickType_t);
int xTaskNotifyWaitIndexed(unsigned, uint32_t, uint32_t, uint32_t *, TickType_t);
void xTaskNotifyIndexedFromISR(TaskHandle_t, unsigned, uint32_t, int, void *);
void vTaskNotifyGiveIndexedFromISR(TaskHandle_t, unsigned, void *);
