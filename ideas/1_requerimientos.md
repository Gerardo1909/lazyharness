# Requerimientos del sistema 

## Historias de usuario

1. Como usuario quiero poder entrar y ver un menu con todos los harneses que armé al ingresar

2. Como usuario quiero poder generar mi esquema de prompts (harness) y poder guardarlos con un nombre


## Requerimientos funcionales

1. Los prompts deben poder ser versionados, es decir que cuando alguien guarda un esquema de prompts bajo un nombre
se inicia un repo de git. Cada guardado es un commit, al usuario antes de guardar se le pide que deje un mensaje

2. Se debe poder volver a una versión anterior del esquema de prompts. Preferiria que los cambios sean a nivel prompt/rol
dentro del harness y asi tener una mayor granularidad, asi si querés volver atrás no tenés efectos secundarios

3. Se debe poder conectar un provider o IA local para que los prompts empiecen a funcionar

4. Si yo selecciono un harness, la pantalla cambia para mostrarme en la barra lateral izquierda todos los roles que tengo y poder desplazarme
entre cada uno para decidir cual quiero llamar. 

5. Al tener un prompt/rol seleccionado, entonces el click se me va a la parte izquierda en donde se setea por defecto al principio del prompt 
que vas a escribir y se te muestra el provider, es como un enlace o mini integracion del modelo que estes usando

6. Debes poder referenciar a otros agentes en partes especificas del prompt y cuando los mencionas, el nombre del rol se formatea y toma un color

7. Se le debe poder asignar colorcitos a los roles y definir jerarquías. Usualmente cuando trabajo con esto, mas allá de los roles y el prompt
que vos podas llegar a generar, siempre genero un archivo de workflow que explica el orden natural por más que no siempre se respeta. Por ejemplo,
en un flujo de desarrollo tenés un prompt que es el arquitecto, que por naturaleza es el lider y este va invocando al code reviewer para que 
revise su plan de implementación y luego invoca subagentes devs. Pero vos también podés invocar unicamente al code reviewer cuando es una feature que hiciste vos y asi tenes más agilidad, digamos que es una jerarquía sugerida.

8. Al usuario se le debe preguntar en que formato desea guardar sus prompts (xml como recomendado, .md, .txt, etc). Luego en base a eso
se abre un editor de texto tipo nano en donde se habilita para escribir. Debe existir una opción "Querés hacerlo con IA?" y que esta habilite
una barra inferior dentro de ese mismo espacio de escritura en donde vos le indicas mediante un prompt al agente tus especificaciones. Por 
detrás al habilitar esa opción, al modelo se le inyecta una skill de harness engineering (esto es invisible al usuario) y se va habilitando
un recuadro con preguntas según corresponda para que el usuario vaya profundizando en lo que quiere. Luego en la barra derecha este va viendo
como se van poblando los prompts y puede ir yendo de uno en uno para editarlos.

9. Por defecto cuando vos te paras sobre un prompt tenes modo de solo lectura y debajo del texto del prompt en una barra inferior, en la misma 
ubicacion en la cual se habilitaria el mini chat con un llm, se te muestran las opciones de: 1. editar, 2. eliminar, 3. mejorar (esta al presionarla lo que hace es que un agente se lanza en background y revisa todos los prompts para optimizarlos, alinearlos, etc). Cada opción
debe mostrar un cuadro de texto pequeño al pararse sobre la misma que explique lo que hace, sus potenciales riesgos, etc.
