<template>
    <div class="Box mb-3">
        <div class="Box-header">
            <h3 class="Box-title f4-light">Deployments <span class="Counter Counter--gray-dark">{{ deployments.length }}</span></h3>
        </div>
        <div v-for="deploy in deployments" class="Box-body d-flex flex-items-center" v-bind:key="deploy">
          <div class="flex-auto">
            <strong>{{ deploy.name }}</strong>
            <div class="text-small text-gray-light">
              {{ deploy.id }}
            </div>
          </div>
          <span class="State State--green">{{ deploy.namespace }}</span>
        </div>
    </div>
</template>

<script>
import axios from 'axios';

export default {
  data() {
    return {
      deployments: [],
      errors: []
    }
  },
  created() {
    axios.get(`http://localhost:30445/deployments`)
    .then(response => {
      this.deployments = response.data
    })
    .catch(e => {
      this.errors.push(e)
    })
  }
}
</script>
