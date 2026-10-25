#pragma once
#include <stdint.h>
#include <stdbool.h>
typedef unsigned uint;
typedef int alarm_id_t;
typedef uint64_t absolute_time_t;
typedef int64_t (*alarm_callback_t)(alarm_id_t, void *);
#define GPIO_IN 0
#define GPIO_OUT 1
#define GPIO_IRQ_EDGE_RISE 1
#define GPIO_IRQ_EDGE_FALL 2
#define IO_IRQ_BANK0 0
uint64_t time_us_64(void);
void gpio_set_dir(uint, bool);
bool gpio_get(uint);
void gpio_acknowledge_irq(uint,uint32_t);
void gpio_set_irq_enabled(uint,uint32_t,bool);
void gpio_set_irq_callback(void (*)(uint,uint32_t));
void gpio_init(uint);
void gpio_disable_pulls(uint);
void gpio_put(uint,bool);
void irq_set_enabled(uint,bool);
alarm_id_t add_alarm_at(absolute_time_t,alarm_callback_t,void *,bool);
alarm_id_t add_alarm_in_us(uint64_t,alarm_callback_t,void *,bool);
bool cancel_alarm(alarm_id_t);
static inline absolute_time_t from_us_since_boot(uint64_t t) { return t; }
