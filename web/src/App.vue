<template>
  <div id="app">
    <div class="header">
      <img id="logo" class="mx-auto text-center" alt="Registeel" src="./assets/logo.png">
    </div>
    <Deployments/>
  </div>
</template>

<script>
import Deployments from './components/Deployments.vue'

export default {
  name: 'app',
  components: {
    Deployments
  }
}
</script>

<style>
#app {
  font-family: 'Avenir', Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  color: #2c3e50;
  margin-top: 60px;
}
.header {
  text-align: center;
}
</style>
